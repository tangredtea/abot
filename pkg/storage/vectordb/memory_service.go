package vectordb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"

	"abot/pkg/types"
)

// MemoryService implements memory.Service using VectorStore + Embedder.
// Multi-tenant: per-tenant collection, user_id filtering.
type MemoryService struct {
	store    types.VectorStore
	embedder Embedder
}

// NewMemoryService creates a vector-backed memory service.
func NewMemoryService(store types.VectorStore, embedder Embedder) *MemoryService {
	return &MemoryService{store: store, embedder: embedder}
}

// CollectionName returns the per-tenant collection name.
func CollectionName(tenantID string) string {
	return fmt.Sprintf("tenant_%s", tenantID)
}

// AddSession extracts text from session events, embeds, and upserts.
func (ms *MemoryService) AddSession(ctx context.Context, s session.Session) error {
	tenantID := stateString(s.State(), "tenant_id")
	if tenantID == "" {
		tenantID = s.AppName()
	}
	userID := stateString(s.State(), "user_id")
	col := CollectionName(tenantID)

	if err := ms.store.EnsureCollection(ctx, col); err != nil {
		return err
	}

	var chunks []string
	var timestamps []time.Time
	for ev := range s.Events().All() {
		text := ExtractEventText(ev)
		if text == "" {
			continue
		}
		chunks = append(chunks, text)
		timestamps = append(timestamps, ev.Timestamp)
	}
	if len(chunks) == 0 {
		return nil
	}

	vecs, err := ms.embedder.Embed(ctx, chunks)
	if err != nil {
		return err
	}

	entries := make([]types.VectorEntry, len(chunks))
	for i := range chunks {
		entries[i] = types.VectorEntry{
			ID:     fmt.Sprintf("%s_%s_%d", s.ID(), s.UserID(), i),
			Vector: vecs[i],
			Payload: map[string]any{
				"text":       chunks[i],
				"user_id":    userID,
				"session_id": s.ID(),
				"timestamp":  timestamps[i].Format(time.RFC3339),
			},
		}
	}
	return ms.store.Upsert(ctx, col, entries)
}

// Search returns memory entries matching the query.
func (ms *MemoryService) Search(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	col := CollectionName(req.AppName)

	vecs, err := ms.embedder.Embed(ctx, []string{req.Query})
	if err != nil {
		return nil, err
	}

	filter := map[string]any{}
	if req.UserID != "" {
		filter["user_id"] = req.UserID
	}

	results, err := ms.store.Search(ctx, col, &types.VectorSearchRequest{
		Vector: vecs[0],
		Filter: filter,
		TopK:   10,
	})
	if err != nil {
		return nil, err
	}

	resp := &memory.SearchResponse{}
	for _, r := range results {
		text, ok := r.Payload["text"].(string)
		if !ok || text == "" {
			continue
		}
		ts, _ := r.Payload["timestamp"].(string)
		t, _ := time.Parse(time.RFC3339, ts)
		resp.Memories = append(resp.Memories, memory.Entry{
			Content:   genai.NewContentFromText(text, "model"),
			Author:    "memory",
			Timestamp: t,
		})
	}
	return resp, nil
}

// stateString extracts a string value from session state, returning "" on missing/wrong type.
func stateString(state session.State, key string) string {
	v, err := state.Get(key)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// ExtractEventText pulls text from a session event's content parts.
func ExtractEventText(ev *session.Event) string {
	if ev.LLMResponse.Content == nil {
		return ""
	}
	var parts []string
	for _, p := range ev.LLMResponse.Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

var _ memory.Service = (*MemoryService)(nil)
