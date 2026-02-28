package memoryconsolidation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"abot/pkg/types"
)

func (c *consolidator) persist(ctx context.Context, tenantID, userID string, entries []MemoryEntry) {
	if len(entries) == 0 {
		return
	}

	collection := fmt.Sprintf("tenant_%s", tenantID)
	if err := c.vectorStore.EnsureCollection(ctx, collection); err != nil {
		slog.Error("memoryconsolidation: ensure collection failed", "err", err)
		return
	}

	for _, entry := range entries {
		if entry.Scope == "user" && userID == "" {
			continue
		}
		c.persistEntry(ctx, collection, tenantID, userID, entry)
	}
}

// persistEntry embeds the entry text, searches for similar existing entries,
// marks superseded ones, and upserts the new entry.
func (c *consolidator) persistEntry(ctx context.Context, collection, tenantID, userID string, entry MemoryEntry) {
	vecs, err := c.embedder.Embed(ctx, []string{entry.Text})
	if err != nil {
		slog.Error("memoryconsolidation: embed failed", "err", err)
		return
	}
	vec := vecs[0]

	// Search for similar existing entries to dedup.
	filter := map[string]any{
		"superseded": false,
		"scope":      entry.Scope,
	}
	if entry.Scope == "user" {
		filter["user_id"] = userID
	}

	threshold := c.dedupScore
	if threshold <= 0 {
		threshold = defaultDedupScore
	}

	results, err := c.vectorStore.Search(ctx, collection, &types.VectorSearchRequest{
		Vector: vec,
		Filter: filter,
		TopK:   5,
	})
	if err != nil {
		slog.Warn("memoryconsolidation: dedup search failed, inserting anyway", "err", err)
	}

	now := time.Now()
	newID := fmt.Sprintf("mem_%s_%d", tenantID, now.UnixMilli())

	// Mark similar entries as superseded.
	// Use a zero vector for superseded entries — they are excluded from search
	// by the "superseded: false" filter, so the vector value is irrelevant.
	// This avoids corrupting the old entry's embedding with the new entry's vector.
	zeroVec := make([]float32, len(vec))
	for _, r := range results {
		if r.Score < threshold {
			continue
		}
		superseded := r.Payload
		if superseded == nil {
			superseded = map[string]any{}
		}
		superseded["superseded"] = true
		superseded["superseded_by"] = newID
		superseded["superseded_at"] = now.Format(time.RFC3339)

		if err := c.vectorStore.Upsert(ctx, collection, []types.VectorEntry{{
			ID:      r.ID,
			Vector:  zeroVec,
			Payload: superseded,
		}}); err != nil {
			slog.Warn("memoryconsolidation: failed to mark superseded", "id", r.ID, "err", err)
		}
	}

	// Upsert the new entry.
	perm := isPermanent(entry)
	payload := map[string]any{
		"text":         entry.Text,
		"category":     entry.Category,
		"scope":        entry.Scope,
		"superseded":   false,
		"permanent":    perm,
		"access_count": 0,
		"created_at":   now.Format(time.RFC3339),
		"source":       "consolidation",
	}
	if userID != "" && entry.Scope == "user" {
		payload["user_id"] = userID
	}

	if err := c.vectorStore.Upsert(ctx, collection, []types.VectorEntry{{
		ID:      newID,
		Vector:  vec,
		Payload: payload,
	}}); err != nil {
		slog.Error("memoryconsolidation: upsert failed", "err", err)
	}
}

// permanentCategories mirrors tools.permanentCategories.
var permanentCategories = map[string]bool{
	"preference":  true,
	"identity":    true,
	"instruction": true,
	"fact":        true,
}

// isPermanent determines whether a memory entry should be permanent.
// Explicit override > category default.
func isPermanent(e MemoryEntry) bool {
	if e.Permanent != nil {
		return *e.Permanent
	}
	return permanentCategories[e.Category]
}
