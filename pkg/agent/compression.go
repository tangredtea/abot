package agent

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
)

// Compressor handles session compression when context grows too large.
type Compressor struct {
	summaryLLM     model.LLM
	sessionService session.Service
	appName        string
}

// NewCompressor creates a compressor with the given summary model.
func NewCompressor(llm model.LLM, ss session.Service, appName string) *Compressor {
	return &Compressor{
		summaryLLM:     llm,
		sessionService: ss,
		appName:        appName,
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

// Compress creates a summary of old events and replaces the session.
// ADK sessions are append-only, so we delete + recreate with summary as first event.
func (c *Compressor) Compress(ctx context.Context, sess session.Session) error {
	events := sess.Events()
	n := events.Len()
	if n <= minEventsToCompress {
		return nil
	}

	// Keep the most recent events based on keepRecentPercent.
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
	prompt := "Summarize the following conversation concisely, preserving key facts, decisions, and context:\n\n" + text

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
