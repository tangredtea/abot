package memoryconsolidation

import (
	"context"
	"fmt"
	"log/slog"

	"abot/pkg/types"
)

const defaultDedupScore = float32(0.85)

// loadVectorMemories retrieves existing active memories from vector store.
// If userID is empty, loads tenant-scope memories only.
func (c *consolidator) loadVectorMemories(ctx context.Context, tenantID, userID string) []VectorMemory {
	collection := fmt.Sprintf("tenant_%s", tenantID)

	filter := map[string]any{
		"superseded": false,
	}
	if userID != "" {
		filter["scope"] = "user"
		filter["user_id"] = userID
	} else {
		filter["scope"] = "tenant"
	}

	// Use a zero vector to get all entries matching the filter.
	// TopK=100 is a reasonable cap for context.
	results, err := c.vectorStore.Search(ctx, collection, &types.VectorSearchRequest{
		Vector: make([]float32, c.embedder.Dimension()),
		Filter: filter,
		TopK:   100,
	})
	if err != nil {
		slog.Debug("memoryconsolidation: load vector memories failed", "err", err)
		return nil
	}

	var memories []VectorMemory
	for _, r := range results {
		memories = append(memories, VectorMemory{
			ID:       r.ID,
			Category: payloadStr(r.Payload, "category"),
			Text:     payloadStr(r.Payload, "text"),
			Scope:    payloadStr(r.Payload, "scope"),
		})
	}
	return memories
}

func payloadStr(p map[string]any, key string) string {
	v, _ := p[key].(string)
	return v
}
