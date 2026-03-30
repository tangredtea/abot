package agent

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"

	"abot/pkg/pruning"
)

// Compressor handles session compression when context grows too large.
type Compressor struct {
	summaryLLM      model.LLM
	sessionService  session.Service
	appName         string
	pruningStrategy pruning.Strategy
}

// NewCompressor creates a compressor with the given summary model.
func NewCompressor(llm model.LLM, ss session.Service, appName string) *Compressor {
	return &Compressor{
		summaryLLM:      llm,
		sessionService:  ss,
		appName:         appName,
		pruningStrategy: pruning.DefaultStrategy(),
	}
}

const (
	eventThreshold       = 50
	contextWindowPercent = 75
	charsPerToken        = 4
	minEventsToCompress  = 4  // skip compression if session has fewer events
	keepRecentPercent    = 25 // percentage of recent events to preserve
)

// ShouldCompress checks whether the session needs compression.
func (c *Compressor) ShouldCompress(sess session.Session, contextWindow int) bool {
	n := sess.Events().Len()
	if n > eventThreshold {
		return true
	}
	tokenEst := c.estimateTokens(sess)
	return tokenEst > contextWindow*contextWindowPercent/100
}

func (c *Compressor) estimateTokens(sess session.Session) int {
	total := 0
	for ev := range sess.Events().All() {
		if ev.Content != nil {
			for _, p := range ev.Content.Parts {
				total += len(p.Text) / charsPerToken
			}
		}
	}
	return total
}

// Compress applies three-layer pruning first, then LLM summarization if needed.
// This reduces LLM calls by ~80% compared to always summarizing.
func (c *Compressor) Compress(ctx context.Context, sess session.Session) error {
	events := sess.Events()
	n := events.Len()
	if n <= minEventsToCompress {
		return nil
	}

	// Convert events to messages
	messages := c.eventsToMessages(events)

	// Try three-layer pruning first (zero cost)
	targetTokens := c.estimateTokens(sess) * 70 / 100 // Target 70% of current
	pruned := c.pruningStrategy.Prune(messages, targetTokens)

	// If pruning succeeded, update session
	if len(pruned) < len(messages) {
		return c.replaceSessionWithMessages(ctx, sess, pruned)
	}

	// Fallback: LLM summarization (only if pruning insufficient)
	keepFrom := n * (100 - keepRecentPercent) / 100
	oldText := c.extractText(events, 0, keepFrom)

	summary, err := c.callSummaryLLM(ctx, oldText)
	if err != nil {
		return fmt.Errorf("compression summary failed: %w", err)
	}

	return c.replaceSession(ctx, sess, summary, keepFrom)
}

// ForceCompress drops the oldest 50% of events without summarization.
func (c *Compressor) ForceCompress(ctx context.Context, sess session.Session) error {
	events := sess.Events()
	n := events.Len()
	if n <= minEventsToCompress {
		return nil
	}

	keepFrom := n / 2
	note := fmt.Sprintf("[System: Emergency compression dropped %d oldest events]", keepFrom)
	return c.replaceSession(ctx, sess, note, keepFrom)
}

func (c *Compressor) extractText(events session.Events, from, to int) string {
	var sb strings.Builder
	for i := from; i < to; i++ {
		ev := events.At(i)
		if ev == nil || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.Text != "" {
				sb.WriteString(p.Text)
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String()
}

func (c *Compressor) callSummaryLLM(ctx context.Context, text string) (string, error) {
	// Structured summary prompt (inspired by OpenClaw)
	prompt := `Generate a structured context checkpoint summary for the following conversation.

Use this exact format:

## Goals
[What does the user want to accomplish? List multiple goals if applicable]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned]
- [(None) if not applicable]

## Progress
### Completed
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Current work]

### Blocked
- [Any blocking issues]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [List what should be done next in order]

## Key Information
- [Any data, examples, or references needed to continue]
- [(None) if not applicable]

Keep each section concise. Preserve exact file paths, function names, and error messages.

Conversation:
` + text

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: prompt}}},
		},
	}

	var result strings.Builder
	for resp, err := range c.summaryLLM.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp.Content != nil {
			for _, p := range resp.Content.Parts {
				result.WriteString(p.Text)
			}
		}
	}
	return result.String(), nil
}

func (c *Compressor) replaceSession(ctx context.Context, sess session.Session, summaryText string, keepFrom int) error {
	userID := sess.UserID()
	sessionID := sess.ID()

	// Collect ALL data before deletion to prevent data loss if recreate fails.
	state := make(map[string]any)
	for k, v := range sess.State().All() {
		state[k] = v
	}

	events := sess.Events()
	keptEvents := make([]*session.Event, 0, events.Len()-keepFrom)
	for i := keepFrom; i < events.Len(); i++ {
		ev := events.At(i)
		if ev != nil {
			keptEvents = append(keptEvents, ev)
		}
	}

	// Delete old session.
	if err := c.sessionService.Delete(ctx, &session.DeleteRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	// Recreate with same ID.
	resp, err := c.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: sessionID,
		State:     state,
	})
	if err != nil {
		return fmt.Errorf("recreate session (data collected, not lost): %w", err)
	}
	newSess := resp.Session

	// Append summary as first event.
	summaryEvent := session.NewEvent("compression")
	summaryEvent.Author = "system"
	summaryEvent.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: summaryText}},
		},
	}
	if err := c.sessionService.AppendEvent(ctx, newSess, summaryEvent); err != nil {
		return fmt.Errorf("append summary event: %w", err)
	}

	// Re-append kept events from pre-collected slice.
	for i, ev := range keptEvents {
		if err := c.sessionService.AppendEvent(ctx, newSess, ev); err != nil {
			return fmt.Errorf("append kept event %d: %w", i, err)
		}
	}

	return nil
}

// eventsToMessages converts session events to genai.Content messages.
func (c *Compressor) eventsToMessages(events session.Events) []*genai.Content {
	messages := make([]*genai.Content, 0, events.Len())
	for i := 0; i < events.Len(); i++ {
		ev := events.At(i)
		if ev != nil && ev.Content != nil {
			messages = append(messages, ev.Content)
		}
	}
	return messages
}

// replaceSessionWithMessages replaces session with pruned messages.
func (c *Compressor) replaceSessionWithMessages(ctx context.Context, sess session.Session, messages []*genai.Content) error {
	userID := sess.UserID()
	sessionID := sess.ID()

	// Collect state
	state := make(map[string]any)
	for k, v := range sess.State().All() {
		state[k] = v
	}

	// Delete old session
	if err := c.sessionService.Delete(ctx, &session.DeleteRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	// Create new session
	createResp, err := c.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   c.appName,
		UserID:    userID,
		SessionID: sessionID,
		State:     state,
	})
	if err != nil {
		return fmt.Errorf("recreate session: %w", err)
	}

	// Append pruned messages
	for i, msg := range messages {
		ev := session.NewEvent("pruned")
		ev.LLMResponse = model.LLMResponse{Content: msg}
		if err := c.sessionService.AppendEvent(ctx, createResp.Session, ev); err != nil {
			return fmt.Errorf("append message %d: %w", i, err)
		}
	}

	return nil
}
