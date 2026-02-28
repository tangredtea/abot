package memoryconsolidation

import (
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"

	"abot/pkg/types"
)

// MemoryEntry is a single categorized memory extracted by the LLM.
type MemoryEntry struct {
	Category  string // free-form: "preference", "fact", "event", "instruction", ...
	Text      string // the memory content
	Scope     string // "tenant" or "user"
	Permanent *bool  // explicit override; nil means auto-determine by category
}

type consolidator struct {
	llm              model.LLM
	vectorStore      types.VectorStore
	embedder         types.Embedder
	memoryEventStore types.MemoryEventStore // optional
	threshold        int
	dedupScore       float32
}

func stateString(s session.State, key string) string {
	v, err := s.Get(key)
	if err != nil {
		return ""
	}
	str, _ := v.(string)
	return str
}

func (c *consolidator) afterRun(ctx agent.InvocationContext) {
	sess := ctx.Session()
	if sess == nil {
		return
	}
	if sess.Events().Len() < c.threshold {
		return
	}

	tenantID := stateString(sess.State(), "tenant_id")
	userID := stateString(sess.State(), "user_id")
	if tenantID == "" {
		return
	}

	conversation := c.extractConversation(sess)
	if conversation == "" {
		return
	}

	// Load existing memories from vector store for context.
	existingTenant := c.loadVectorMemories(ctx, tenantID, "")
	var existingUser []VectorMemory
	if userID != "" {
		existingUser = c.loadVectorMemories(ctx, tenantID, userID)
	}

	entries, err := c.consolidate(ctx, conversation, existingTenant, existingUser, userID != "")
	if err != nil {
		slog.Error("memoryconsolidation: consolidation failed", "err", err)
		return
	}

	c.persist(ctx, tenantID, userID, entries)

	// Write consolidation event to event log.
	c.writeEventLog(ctx, tenantID, userID, entries)
}

func (c *consolidator) writeEventLog(ctx agent.InvocationContext, tenantID, userID string, entries []MemoryEntry) {
	if c.memoryEventStore == nil || len(entries) == 0 {
		return
	}
	summary := fmt.Sprintf("Consolidated %d memories", len(entries))
	if err := c.memoryEventStore.Add(ctx, &types.MemoryEvent{
		TenantID:  tenantID,
		UserID:    userID,
		Category:  "consolidation",
		Summary:   summary,
		CreatedAt: time.Now(),
	}); err != nil {
		slog.Warn("memoryconsolidation: failed to write event log", "err", err)
	}
}
