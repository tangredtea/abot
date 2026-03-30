// Package search provides hybrid search combining BM25 keyword and vector search.
//
// Inspired by OpenClaw's memory system:
// - BM25 keyword search (TF-IDF based)
// - Vector semantic search
// - Weighted hybrid scoring
package search

import (
	"context"
	"sort"
)

// HybridSearcher combines keyword and vector search.
type HybridSearcher struct {
	vectorStore VectorStore
	bm25        *BM25Index
	weights     SearchWeights
}

// SearchWeights defines scoring weights for hybrid search.
type SearchWeights struct {
	Keyword float64 // BM25 weight (default 0.3)
	Vector  float64 // Vector weight (default 0.7)
}

// VectorStore interface for vector search backend.
type VectorStore interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// SearchResult represents a search result with score.
type SearchResult struct {
	ID      string
	Content string
	Score   float64
}

// NewHybridSearcher creates a hybrid searcher.
func NewHybridSearcher(vectorStore VectorStore, bm25 *BM25Index) *HybridSearcher {
	return &HybridSearcher{
		vectorStore: vectorStore,
		bm25:        bm25,
		weights: SearchWeights{
			Keyword: 0.3,
			Vector:  0.7,
		},
	}
}

// Search performs hybrid search combining keyword and vector results.
func (h *HybridSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// 1. Keyword search (BM25)
	keywordResults := h.bm25.Search(query, limit*2)

	// 2. Vector search
	vectorResults, err := h.vectorStore.Search(ctx, query, limit*2)
	if err != nil {
		return nil, err
	}

	// 3. Merge and re-rank
	merged := h.mergeResults(keywordResults, vectorResults)

	// 4. Return top N
	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged, nil
}

// mergeResults combines keyword and vector results with weighted scoring.
func (h *HybridSearcher) mergeResults(keyword, vector []SearchResult) []SearchResult {
	// Normalize scores to [0, 1]
	keyword = normalizeScores(keyword)
	vector = normalizeScores(vector)

	// Merge by ID
	scoreMap := make(map[string]*SearchResult)

	for _, r := range keyword {
		scoreMap[r.ID] = &SearchResult{
			ID:      r.ID,
			Content: r.Content,
			Score:   r.Score * h.weights.Keyword,
		}
	}

	for _, r := range vector {
		if existing, ok := scoreMap[r.ID]; ok {
			existing.Score += r.Score * h.weights.Vector
		} else {
			scoreMap[r.ID] = &SearchResult{
				ID:      r.ID,
				Content: r.Content,
				Score:   r.Score * h.weights.Vector,
			}
		}
	}

	// Convert to slice and sort
	results := make([]SearchResult, 0, len(scoreMap))
	for _, r := range scoreMap {
		results = append(results, *r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// normalizeScores normalizes scores to [0, 1] range.
func normalizeScores(results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	maxScore := results[0].Score
	minScore := results[len(results)-1].Score

	if maxScore == minScore {
		return results
	}

	normalized := make([]SearchResult, len(results))
	for i, r := range results {
		normalized[i] = SearchResult{
			ID:      r.ID,
			Content: r.Content,
			Score:   (r.Score - minScore) / (maxScore - minScore),
		}
	}

	return normalized
}
