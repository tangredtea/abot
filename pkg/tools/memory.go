package tools

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"abot/pkg/types"

	"github.com/google/uuid"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Scoring weights for mixed retrieval.
const (
	wSemantic = 0.60
	wRecency  = 0.25
	wSalience = 0.15
	// Half-life for recency decay (days). After 7 days, score ≈ 0.5.
	recencyHalfLifeDays = 7.0

	// Dedup thresholds for save_memory.
	dedupSameCategory = float32(0.70) // lower bar when category matches
	dedupCrossCategory = float32(0.85) // higher bar across categories
)

// permanentCategories are categories that default to permanent=true.
var permanentCategories = map[string]bool{
	"preference":  true,
	"identity":    true,
	"instruction": true,
	"fact":        true,
}

// recencyScore returns a time-decay score in [0, 1].
// Permanent memories always return 1.0.
func recencyScore(createdAt time.Time, permanent bool) float64 {
	if permanent {
		return 1.0
	}
	days := time.Since(createdAt).Hours() / 24.0
	if days < 0 {
		days = 0
	}
	lambda := math.Ln2 / recencyHalfLifeDays
	return math.Exp(-lambda * days)
}

// salienceScore returns an importance score in [0, 1] based on access count.
// Uses log scale: 0→0, 3→~0.4, 31→1.0.
func salienceScore(accessCount int) float64 {
	if accessCount <= 0 {
		return 0
	}
	return math.Min(math.Log2(float64(accessCount)+1)/5.0, 1.0)
}

// isPermanentDefault returns whether a category defaults to permanent.
func isPermanentDefault(category string) bool {
	return permanentCategories[category]
}

// --- save_memory ---

type saveMemoryArgs struct {
	Content   string `json:"content" jsonschema:"The memory content to save"`
	Category  string `json:"category,omitempty" jsonschema:"Category: preference/fact/event/instruction/identity/goal/general"`
	Permanent *bool  `json:"permanent,omitempty" jsonschema:"Mark as permanent (never decays). Auto-determined by category if omitted."`
}

type saveMemoryResult struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newSaveMemory(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "save_memory",
		Description: "Save a piece of information to long-term memory. Use when the user asks you to remember something or when you encounter important facts worth preserving.",
	}, func(ctx tool.Context, args saveMemoryArgs) (saveMemoryResult, error) {
		if deps.VectorStore == nil || deps.Embedder == nil {
			return saveMemoryResult{Error: "memory storage not configured"}, nil
		}
		if strings.TrimSpace(args.Content) == "" {
			return saveMemoryResult{Error: "content is empty"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}
		userID := stateStr(ctx, "user_id")

		collection := fmt.Sprintf("tenant_%s", tenantID)
		if err := deps.VectorStore.EnsureCollection(ctx, collection); err != nil {
			return saveMemoryResult{Error: fmt.Sprintf("ensure collection: %v", err)}, nil
		}

		vecs, err := deps.Embedder.Embed(ctx, []string{args.Content})
		if err != nil {
			return saveMemoryResult{Error: fmt.Sprintf("embed: %v", err)}, nil
		}

		category := args.Category
		if category == "" {
			category = "general"
		}
		scope := "user"
		if userID == "" {
			scope = "tenant"
		}

		// Determine permanent: explicit param > category default.
		perm := isPermanentDefault(category)
		if args.Permanent != nil {
			perm = *args.Permanent
		}

		now := time.Now()
		id := uuid.New().String()

		// Dedup: search for similar existing memories and supersede them.
		dedupFilter := map[string]any{
			"superseded": false,
			"scope":      scope,
		}
		if scope == "user" {
			dedupFilter["user_id"] = userID
		}
		if similar, err := deps.VectorStore.Search(ctx, collection, &types.VectorSearchRequest{
			Vector: vecs[0],
			Filter: dedupFilter,
			TopK:   5,
		}); err == nil {
			zeroVec := make([]float32, len(vecs[0]))
			for _, s := range similar {
				oldCat, _ := s.Payload["category"].(string)
				threshold := dedupCrossCategory
				if oldCat == category {
					threshold = dedupSameCategory
				}
				if s.Score < threshold {
					continue
				}
				sup := s.Payload
				if sup == nil {
					sup = map[string]any{}
				}
				oldText, _ := s.Payload["text"].(string)
				slog.Debug("save_memory: superseding old memory",
					"old_id", s.ID,
					"old_text", Truncate(oldText, 60),
					"old_category", oldCat,
					"score", s.Score,
					"new_id", id,
				)
				sup["superseded"] = true
				sup["superseded_by"] = id
				sup["superseded_at"] = now.Format(time.RFC3339)
				_ = deps.VectorStore.Upsert(ctx, collection, []types.VectorEntry{{
					ID: s.ID, Vector: zeroVec, Payload: sup,
				}})
			}
		}

		payload := map[string]any{
			"text":         args.Content,
			"category":     category,
			"scope":        scope,
			"superseded":   false,
			"permanent":    perm,
			"access_count": 0,
			"created_at":   now.Format(time.RFC3339),
			"source":       "user",
		}
		if userID != "" {
			payload["user_id"] = userID
		}

		if err := deps.VectorStore.Upsert(ctx, collection, []types.VectorEntry{{
			ID:      id,
			Vector:  vecs[0],
			Payload: payload,
		}}); err != nil {
			return saveMemoryResult{Error: fmt.Sprintf("upsert: %v", err)}, nil
		}

		return saveMemoryResult{Result: fmt.Sprintf("saved memory [%s]: %s", category, Truncate(args.Content, 80))}, nil
	})
	return t
}

// --- search_memory ---

type searchMemoryArgs struct {
	Query    string `json:"query" jsonschema:"Search query to find relevant memories"`
	DateFrom string `json:"date_from,omitempty" jsonschema:"Filter memories created after this date (RFC3339 or YYYY-MM-DD)"`
	DateTo   string `json:"date_to,omitempty" jsonschema:"Filter memories created before this date (RFC3339 or YYYY-MM-DD)"`
}

type searchMemoryResult struct {
	Memories []memoryHit `json:"memories,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type memoryHit struct {
	Category  string  `json:"category"`
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	CreatedAt string  `json:"created_at,omitempty"`
	Permanent bool    `json:"permanent,omitempty"`
}

func newSearchMemory(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "search_memory",
		Description: "Search long-term memory for relevant information. Use when the user asks about past conversations, saved facts, or previously remembered information. Supports date filtering.",
	}, func(ctx tool.Context, args searchMemoryArgs) (searchMemoryResult, error) {
		if deps.VectorStore == nil || deps.Embedder == nil {
			return searchMemoryResult{Error: "memory storage not configured"}, nil
		}
		if strings.TrimSpace(args.Query) == "" {
			return searchMemoryResult{Error: "query is empty"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}
		userID := stateStr(ctx, "user_id")

		collection := fmt.Sprintf("tenant_%s", tenantID)

		vecs, err := deps.Embedder.Embed(ctx, []string{args.Query})
		if err != nil {
			return searchMemoryResult{Error: fmt.Sprintf("embed: %v", err)}, nil
		}

		// Two-pass search: tenant-scope + user-scope, merged for re-ranking.
		tenantFilter := map[string]any{
			"superseded": false,
			"scope":      "tenant",
		}
		userFilter := map[string]any{
			"superseded": false,
			"scope":      "user",
		}
		if userID != "" {
			userFilter["user_id"] = userID
		}

		tenantResults, err := deps.VectorStore.Search(ctx, collection, &types.VectorSearchRequest{
			Vector: vecs[0],
			Filter: tenantFilter,
			TopK:   15,
		})
		if err != nil {
			return searchMemoryResult{Error: fmt.Sprintf("search tenant: %v", err)}, nil
		}
		userResults, err := deps.VectorStore.Search(ctx, collection, &types.VectorSearchRequest{
			Vector: vecs[0],
			Filter: userFilter,
			TopK:   15,
		})
		if err != nil {
			return searchMemoryResult{Error: fmt.Sprintf("search user: %v", err)}, nil
		}
		results := append(tenantResults, userResults...)

		// Parse optional date bounds.
		dateFrom, dateTo := parseDateBounds(args.DateFrom, args.DateTo)

		var hits []memoryHit
		for _, r := range results {
			text, _ := r.Payload["text"].(string)
			cat, _ := r.Payload["category"].(string)
			if text == "" {
				continue
			}

			createdStr, _ := r.Payload["created_at"].(string)
			createdAt, _ := time.Parse(time.RFC3339, createdStr)
			perm, _ := r.Payload["permanent"].(bool)
			accessCnt := payloadInt(r.Payload, "access_count")

			// Apply date filter.
			if !dateFrom.IsZero() && createdAt.Before(dateFrom) {
				continue
			}
			if !dateTo.IsZero() && createdAt.After(dateTo) {
				continue
			}

			// Mixed scoring: semantic + recency + salience.
			semantic := float64(r.Score)
			recency := recencyScore(createdAt, perm)
			salience := salienceScore(accessCnt)
			mixed := wSemantic*semantic + wRecency*recency + wSalience*salience

			hits = append(hits, memoryHit{
				Category:  cat,
				Text:      text,
				Score:     mixed,
				CreatedAt: createdStr,
				Permanent: perm,
			})
		}

		// Sort by mixed score descending.
		sort.Slice(hits, func(i, j int) bool {
			return hits[i].Score > hits[j].Score
		})

		// Cap to top 10.
		if len(hits) > 10 {
			hits = hits[:10]
		}

		if len(hits) == 0 {
			return searchMemoryResult{Memories: []memoryHit{}}, nil
		}
		return searchMemoryResult{Memories: hits}, nil
	})
	return t
}

// parseDateBounds parses optional date strings into time.Time.
// Accepts RFC3339 or YYYY-MM-DD formats.
func parseDateBounds(from, to string) (time.Time, time.Time) {
	var f, t time.Time
	if from != "" {
		f, _ = time.Parse(time.RFC3339, from)
		if f.IsZero() {
			f, _ = time.Parse("2006-01-02", from)
		}
	}
	if to != "" {
		t, _ = time.Parse(time.RFC3339, to)
		if t.IsZero() {
			if d, err := time.Parse("2006-01-02", to); err == nil {
				t = d.Add(24*time.Hour - time.Nanosecond)
			}
		}
	}
	return f, t
}

// payloadInt extracts an int from a payload map, handling JSON number types.
func payloadInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// --- delete_memory ---

type deleteMemoryArgs struct {
	Query    string `json:"query,omitempty" jsonschema:"Search query to find memories to delete. Either query or clear_all is required."`
	ClearAll bool   `json:"clear_all,omitempty" jsonschema:"Set true to delete ALL memories for the current scope. Use with caution."`
}

type deleteMemoryResult struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newDeleteMemory(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "delete_memory",
		Description: "Delete memories from vector store. Provide a query to find and delete specific memories, or set clear_all=true to wipe all memories for the current tenant.",
	}, func(ctx tool.Context, args deleteMemoryArgs) (deleteMemoryResult, error) {
		if deps.VectorStore == nil || deps.Embedder == nil {
			return deleteMemoryResult{Error: "memory storage not configured"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}
		userID := stateStr(ctx, "user_id")
		collection := fmt.Sprintf("tenant_%s", tenantID)

		// Clear all: scoped to current user's memories only.
		// When userID is present: delete user-scope memories for this user.
		// When userID is empty: delete only tenant-scope memories (no user_id).
		if args.ClearAll {
			filter := map[string]any{"superseded": false}
			if userID != "" {
				filter["user_id"] = userID
			} else {
				filter["scope"] = "tenant"
			}
			if err := deps.VectorStore.Delete(ctx, collection, filter); err != nil {
				return deleteMemoryResult{Error: fmt.Sprintf("delete: %v", err)}, nil
			}
			// Clean up corresponding superseded ones with same scope.
			supFilter := map[string]any{"superseded": true}
			if userID != "" {
				supFilter["user_id"] = userID
			} else {
				supFilter["scope"] = "tenant"
			}
			_ = deps.VectorStore.Delete(ctx, collection, supFilter)
			slog.Info("delete_memory: cleared memories", "tenant", tenantID, "user", userID)
			return deleteMemoryResult{Result: "all memories cleared"}, nil
		}

		// Query-based delete.
		if strings.TrimSpace(args.Query) == "" {
			return deleteMemoryResult{Error: "provide query or set clear_all=true"}, nil
		}

		vecs, err := deps.Embedder.Embed(ctx, []string{args.Query})
		if err != nil {
			return deleteMemoryResult{Error: fmt.Sprintf("embed: %v", err)}, nil
		}

		delFilter := map[string]any{"superseded": false}
		if userID != "" {
			delFilter["user_id"] = userID
		}
		results, err := deps.VectorStore.Search(ctx, collection, &types.VectorSearchRequest{
			Vector: vecs[0],
			Filter: delFilter,
			TopK:   5,
		})
		if err != nil {
			return deleteMemoryResult{Error: fmt.Sprintf("search: %v", err)}, nil
		}

		deleted := 0
		zeroVec := make([]float32, len(vecs[0]))
		for _, r := range results {
			if r.Score < 0.60 {
				continue
			}
			text, _ := r.Payload["text"].(string)
			slog.Info("delete_memory: removing",
				"id", r.ID, "text", Truncate(text, 60), "score", r.Score)
			sup := r.Payload
			sup["superseded"] = true
			sup["superseded_by"] = "deleted"
			_ = deps.VectorStore.Upsert(ctx, collection, []types.VectorEntry{{
				ID: r.ID, Vector: zeroVec, Payload: sup,
			}})
			deleted++
		}

		if deleted == 0 {
			return deleteMemoryResult{Result: "no matching memories found"}, nil
		}
		return deleteMemoryResult{Result: fmt.Sprintf("deleted %d memories", deleted)}, nil
	})
	return t
}

