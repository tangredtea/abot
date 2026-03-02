package agent

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"abot/pkg/types"
)

// maxContextRetries is the upper limit for context overflow retry attempts.
const maxContextRetries = 2

// sessionKeySep is the delimiter for auto-generated session keys.
// ASCII unit separator (0x1f) avoids collision with values containing ":".
const sessionKeySep = "\x1f"

// SessionKey builds a deterministic session key from tenant, user, and channel.
// Exported so tests can construct matching keys.
func SessionKey(tenantID, userID, channel string) string {
	return tenantID + sessionKeySep + userID + sessionKeySep + channel
}

// ErrorHandler is called when a message processing error occurs.
// Implementations can log, emit metrics, or alert. Must be safe for concurrent use.
type ErrorHandler func(msg types.InboundMessage, err error)

// AgentLoop consumes inbound messages, routes to agents, and publishes responses.
type AgentLoop struct {
	bus            types.MessageBus
	registry       *AgentRegistry
	sessionService session.Service
	compressor     *Compressor
	appName        string
	contextWindow  int
	onError        ErrorHandler
}

// NewAgentLoop creates the main processing loop.
// If onError is nil, a default log-based handler is used.
func NewAgentLoop(bus types.MessageBus, reg *AgentRegistry, ss session.Service, comp *Compressor, appName string, ctxWindow int, onError ErrorHandler) *AgentLoop {
	if onError == nil {
		onError = func(_ types.InboundMessage, err error) {
			slog.Error("agent-loop: process error", "err", err)
		}
	}
	return &AgentLoop{
		bus:            bus,
		registry:       reg,
		sessionService: ss,
		compressor:     comp,
		appName:        appName,
		contextWindow:  ctxWindow,
		onError:        onError,
	}
}

// Run is the main loop: consume inbound → route → run agent → publish outbound.
// A panic in a single message will not crash the entire loop.
func (al *AgentLoop) Run(ctx context.Context) error {
	for {
		msg, err := al.bus.ConsumeInbound(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("consume inbound: %w", err)
		}

		if err := al.safeProcessMessage(ctx, msg); err != nil {
			al.onError(msg, err)
		}
	}
}

// safeProcessMessage wraps processMessage with panic recovery to prevent a
// single message from crashing the entire loop.
func (al *AgentLoop) safeProcessMessage(ctx context.Context, msg types.InboundMessage) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic recovered in processMessage: %v", r)
			slog.Error("agent-loop: panic recovered", "err", retErr)
		}
	}()
	return al.processMessage(ctx, msg)
}

func (al *AgentLoop) processMessage(ctx context.Context, msg types.InboundMessage) error {
	agentID := al.registry.ResolveRoute(msg)
	if agentID == "" {
		return fmt.Errorf("no agent found for channel=%s chatID=%s", msg.Channel, msg.ChatID)
	}

	r, ok := al.registry.GetRunner(agentID)
	if !ok {
		return fmt.Errorf("runner not found for agent %q", agentID)
	}

	sessionKey := msg.SessionKey
	if sessionKey == "" {
		sessionKey = SessionKey(msg.TenantID, msg.UserID, msg.Channel)
	}

	// Ensure session exists.
	if _, err := al.ensureSession(ctx, msg, sessionKey); err != nil {
		return err
	}

	// Build user content.
	content := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: msg.Content}},
	}

	// Run agent with automatic compression retry on context overflow.
	response := al.runAgentWithRetry(ctx, r, msg, sessionKey, content)

	// Publish outbound.
	if response != "" {
		slog.Info("agent-loop: publishing response", "channel", msg.Channel, "chat_id", msg.ChatID, "response_len", len(response))
		if err := al.bus.PublishOutbound(ctx, types.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: response,
		}); err != nil {
			return fmt.Errorf("publish outbound: %w", err)
		}
	} else {
		slog.Warn("agent-loop: empty response from agent", "channel", msg.Channel, "chat_id", msg.ChatID)
	}

	// Check compression — re-fetch session to see events added by runAgent.
	if al.compressor != nil {
		freshSess, err := al.sessionService.Get(ctx, &session.GetRequest{
			AppName:   al.appName,
			UserID:    msg.UserID,
			SessionID: sessionKey,
		})
		if err == nil {
			al.maybeCompress(ctx, freshSess.Session)
		}
	}

	return nil
}

func (al *AgentLoop) ensureSession(ctx context.Context, msg types.InboundMessage, sessionKey string) (session.Session, error) {
	resp, err := al.sessionService.Get(ctx, &session.GetRequest{
		AppName:   al.appName,
		UserID:    msg.UserID,
		SessionID: sessionKey,
	})
	if err == nil {
		return resp.Session, nil
	}

	// Session doesn't exist, create it.
	state := map[string]any{
		"tenant_id": msg.TenantID,
		"user_id":   msg.UserID,
		"channel":   msg.Channel,
		"chat_id":   msg.ChatID,
	}
	createResp, err := al.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   al.appName,
		UserID:    msg.UserID,
		SessionID: sessionKey,
		State:     state,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return createResp.Session, nil
}

// runAgentOnce executes a single agent call and returns the response text
// along with the first error encountered.
func (al *AgentLoop) runAgentOnce(ctx context.Context, r *runner.Runner, userID, sessionID string, content *genai.Content) (string, error) {
	var sb strings.Builder
	var firstErr error
	for ev, err := range r.Run(ctx, userID, sessionID, content, adkagent.RunConfig{}) {
		if err != nil {
			firstErr = err
			break
		}
		if ev == nil || ev.Content == nil {
			continue
		}
		if ev.IsFinalResponse() {
			for _, p := range ev.Content.Parts {
				if p.Text != "" {
					sb.WriteString(p.Text)
				}
			}
		}
	}
	return stripThinking(sb.String()), firstErr
}

// thinkingRe matches <think>...</think> and <thinking>...</thinking> blocks (including newlines).
var thinkingRe = regexp.MustCompile(`(?s)<think(?:ing)?>\n?.*?\n?</think(?:ing)?>`)

// stripThinking removes reasoning/thinking blocks from LLM output before sending to users.
func stripThinking(s string) string {
	return strings.TrimSpace(thinkingRe.ReplaceAllString(s, ""))
}

// contextOverflowPatterns are error substrings that reliably indicate context window overflow.
// These match known error codes/messages from OpenAI, Anthropic, and other LLM providers.
var contextOverflowPatterns = []string{
	"context_length_exceeded",
	"max_tokens",
	"maximum context length",
	"token limit",
	"context window",
	"too many tokens",
	"request too large",
	"prompt is too long",
}

// IsContextOverflowError reports whether err indicates a context window
// overflow (token/context limit exceeded).
func IsContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, p := range contextOverflowPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// runAgentWithRetry runs the agent and automatically compresses the session
// on context overflow, retrying up to maxContextRetries times.
func (al *AgentLoop) runAgentWithRetry(ctx context.Context, r *runner.Runner, msg types.InboundMessage, sessionKey string, content *genai.Content) string {
	for retry := 0; retry <= maxContextRetries; retry++ {
		response, err := al.runAgentOnce(ctx, r, msg.UserID, sessionKey, content)
		if err == nil {
			return response
		}

		// Non-context-overflow error, do not retry.
		if !IsContextOverflowError(err) {
			slog.Error("agent-loop: runner error (non-retryable)", "err", err)
			return ""
		}

		// Last retry also failed.
		if retry >= maxContextRetries {
			slog.Error("agent-loop: runner error after retries", "retries", retry, "err", err)
			return ""
		}

		slog.Warn("agent-loop: context overflow, compressing session", "retry", retry+1, "max", maxContextRetries)

		// Try compressing the session before retrying.
		if al.compressor == nil {
			slog.Error("agent-loop: no compressor configured, cannot recover from context overflow")
			return ""
		}

		freshSess, getErr := al.sessionService.Get(ctx, &session.GetRequest{
			AppName:   al.appName,
			UserID:    msg.UserID,
			SessionID: sessionKey,
		})
		if getErr != nil {
			slog.Error("agent-loop: failed to get session for compression", "err", getErr)
			return ""
		}

		// First attempt uses normal compression; fall back to force compression on failure.
		if compErr := al.compressor.Compress(ctx, freshSess.Session); compErr != nil {
			slog.Warn("agent-loop: compression failed, forcing", "err", compErr)
			if forceErr := al.compressor.ForceCompress(ctx, freshSess.Session); forceErr != nil {
				slog.Error("agent-loop: force compression also failed", "err", forceErr)
				return ""
			}
		}
	}
	return ""
}

// ProcessDirect processes a message synchronously and returns the response.
// Unlike processMessage, it skips bus publishing — designed for the agent CLI mode.
func (al *AgentLoop) ProcessDirect(ctx context.Context, msg types.InboundMessage) (string, error) {
	agentID := msg.AgentID
	if agentID == "" {
		agentID = al.registry.ResolveRoute(msg)
	}
	if agentID == "" {
		return "", fmt.Errorf("no agent found for channel=%s chatID=%s", msg.Channel, msg.ChatID)
	}

	r, ok := al.registry.GetRunner(agentID)
	if !ok {
		return "", fmt.Errorf("runner not found for agent %q", agentID)
	}

	sessionKey := msg.SessionKey
	if sessionKey == "" {
		sessionKey = SessionKey(msg.TenantID, msg.UserID, msg.Channel)
	}

	if _, err := al.ensureSession(ctx, msg, sessionKey); err != nil {
		return "", err
	}

	content := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: msg.Content}},
	}

	response := al.runAgentWithRetry(ctx, r, msg, sessionKey, content)

	// Post-run compression check.
	if al.compressor != nil {
		freshSess, err := al.sessionService.Get(ctx, &session.GetRequest{
			AppName:   al.appName,
			UserID:    msg.UserID,
			SessionID: sessionKey,
		})
		if err == nil {
			al.maybeCompress(ctx, freshSess.Session)
		}
	}

	return response, nil
}

func (al *AgentLoop) maybeCompress(ctx context.Context, sess session.Session) {
	if !al.compressor.ShouldCompress(sess, al.contextWindow) {
		return
	}
	if err := al.compressor.Compress(ctx, sess); err != nil {
		slog.Warn("agent-loop: compression failed, forcing", "err", err)
		if err := al.compressor.ForceCompress(ctx, sess); err != nil {
			slog.Error("agent-loop: force compression failed", "err", err)
		}
	}
}

// StreamEvent represents a real-time event during agent processing.
type StreamEvent struct {
	Type    string         `json:"type"`            // "text_delta", "tool_call", "tool_result", "done", "error"
	Content string         `json:"content"`         // text content or tool name
	Args    map[string]any `json:"args,omitempty"`  // tool call arguments
	Error   string         `json:"error,omitempty"` // error message (for error type)
}

// StreamCallback is called for each streaming event during agent processing.
type StreamCallback func(event StreamEvent)

// ProcessDirectStream processes a message with streaming callbacks.
// Unlike ProcessDirect, it sends incremental events via the callback.
func (al *AgentLoop) ProcessDirectStream(ctx context.Context, msg types.InboundMessage, cb StreamCallback) error {
	agentID := msg.AgentID
	if agentID == "" {
		agentID = al.registry.ResolveRoute(msg)
	}
	if agentID == "" {
		cb(StreamEvent{Type: "error", Error: fmt.Sprintf("no agent found for channel=%s chatID=%s", msg.Channel, msg.ChatID)})
		return fmt.Errorf("no agent found for channel=%s chatID=%s", msg.Channel, msg.ChatID)
	}

	r, ok := al.registry.GetRunner(agentID)
	if !ok {
		cb(StreamEvent{Type: "error", Error: fmt.Sprintf("runner not found for agent %q", agentID)})
		return fmt.Errorf("runner not found for agent %q", agentID)
	}

	sessionKey := msg.SessionKey
	if sessionKey == "" {
		sessionKey = SessionKey(msg.TenantID, msg.UserID, msg.Channel)
	}

	if _, err := al.ensureSession(ctx, msg, sessionKey); err != nil {
		cb(StreamEvent{Type: "error", Error: err.Error()})
		return err
	}

	content := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: msg.Content}},
	}

	// Run agent with streaming and retry on context overflow.
	err := al.runAgentStreamWithRetry(ctx, r, msg, sessionKey, content, cb)

	// Post-run compression check.
	if al.compressor != nil {
		freshSess, getErr := al.sessionService.Get(ctx, &session.GetRequest{
			AppName:   al.appName,
			UserID:    msg.UserID,
			SessionID: sessionKey,
		})
		if getErr == nil {
			al.maybeCompress(ctx, freshSess.Session)
		}
	}

	return err
}

// runAgentStreamOnce executes a single streaming agent call, sending events via the callback.
func (al *AgentLoop) runAgentStreamOnce(ctx context.Context, r *runner.Runner, userID, sessionID string, content *genai.Content, cb StreamCallback) error {
	var finalText strings.Builder
	var firstErr error

	for ev, err := range r.Run(ctx, userID, sessionID, content, adkagent.RunConfig{}) {
		if err != nil {
			firstErr = err
			break
		}
		if ev == nil || ev.Content == nil {
			continue
		}

		for _, p := range ev.Content.Parts {
			if p.Text != "" {
				if ev.IsFinalResponse() {
					finalText.WriteString(p.Text)
				} else {
					// Partial text delta.
					cb(StreamEvent{Type: "text_delta", Content: p.Text})
				}
			}
			if p.FunctionCall != nil {
				args := make(map[string]any)
				if p.FunctionCall.Args != nil {
					for k, v := range p.FunctionCall.Args {
						args[k] = v
					}
				}
				cb(StreamEvent{Type: "tool_call", Content: p.FunctionCall.Name, Args: args})
			}
			if p.FunctionResponse != nil {
				respContent := ""
				if p.FunctionResponse.Response != nil {
					if result, ok := p.FunctionResponse.Response["result"]; ok {
						if s, ok := result.(string); ok {
							respContent = s
						}
					}
				}
				cb(StreamEvent{Type: "tool_result", Content: respContent})
			}
		}

		if ev.IsFinalResponse() {
			text := stripThinking(finalText.String())
			cb(StreamEvent{Type: "text_delta", Content: text})
			cb(StreamEvent{Type: "done"})
		}
	}

	return firstErr
}

// runAgentStreamWithRetry runs the streaming agent with automatic compression retry.
func (al *AgentLoop) runAgentStreamWithRetry(ctx context.Context, r *runner.Runner, msg types.InboundMessage, sessionKey string, content *genai.Content, cb StreamCallback) error {
	for retry := 0; retry <= maxContextRetries; retry++ {
		err := al.runAgentStreamOnce(ctx, r, msg.UserID, sessionKey, content, cb)
		if err == nil {
			return nil
		}

		if !IsContextOverflowError(err) {
			cb(StreamEvent{Type: "error", Error: err.Error()})
			return err
		}

		if retry >= maxContextRetries {
			cb(StreamEvent{Type: "error", Error: "context overflow after retries"})
			return err
		}

		slog.Warn("agent-loop: stream context overflow, compressing", "retry", retry+1)

		if al.compressor == nil {
			cb(StreamEvent{Type: "error", Error: "context overflow, no compressor"})
			return err
		}

		freshSess, getErr := al.sessionService.Get(ctx, &session.GetRequest{
			AppName:   al.appName,
			UserID:    msg.UserID,
			SessionID: sessionKey,
		})
		if getErr != nil {
			cb(StreamEvent{Type: "error", Error: "failed to get session for compression"})
			return getErr
		}

		if compErr := al.compressor.Compress(ctx, freshSess.Session); compErr != nil {
			if forceErr := al.compressor.ForceCompress(ctx, freshSess.Session); forceErr != nil {
				cb(StreamEvent{Type: "error", Error: "compression failed"})
				return forceErr
			}
		}
	}
	return nil
}
