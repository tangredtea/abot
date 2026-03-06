// Package compaction provides adaptive chunk-based summarization.
package agent

import (
	"context"
	"fmt"
	"math"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
)

const (
	BaseChunkRatio  = 0.4  // 40% as base chunk size
	MinChunkRatio   = 0.15 // Minimum 15% chunk size
	SafetyMargin    = 1.2  // 20% safety margin
	MaxSummaryTokens = 900 // Max tokens per summary
)

// AdaptiveCompressor provides adaptive chunk-based compression.
type AdaptiveCompressor struct {
	*Compressor
}

// NewAdaptiveCompressor creates an adaptive compressor.
func NewAdaptiveCompressor(llm model.LLM, ss session.Service, appName string) *AdaptiveCompressor {
	return &AdaptiveCompressor{
		Compressor: NewCompressor(llm, ss, appName),
	}
}

// CompressAdaptive performs adaptive chunk-based compression.
func (a *AdaptiveCompressor) CompressAdaptive(ctx context.Context, sess session.Session, contextWindow int) error {
	events := sess.Events()
	n := events.Len()
	if n <= minEventsToCompress {
		return nil
	}

	// Calculate adaptive chunk size
	totalTokens := a.estimateTokens(sess)
	chunkSize := a.calculateChunkSize(totalTokens, contextWindow)
	numChunks := int(math.Ceil(float64(totalTokens) / float64(chunkSize)))

	if numChunks <= 1 {
		// Single chunk, use regular compression
		return a.Compress(ctx, sess)
	}

	// Multi-chunk summarization
	summaries := make([]string, 0, numChunks)
	eventsPerChunk := n / numChunks

	for i := 0; i < numChunks; i++ {
		start := i * eventsPerChunk
		end := start + eventsPerChunk
		if i == numChunks-1 {
			end = n // Last chunk takes remaining
		}

		chunkText := a.extractText(events, start, end)
		summary, err := a.callSummaryLLM(ctx, chunkText)
		if err != nil {
			return fmt.Errorf("chunk %d summary failed: %w", i, err)
		}
		summaries = append(summaries, summary)
	}

	// Merge summaries
	finalSummary, err := a.mergeSummaries(ctx, summaries)
	if err != nil {
		return fmt.Errorf("merge summaries failed: %w", err)
	}

	// Keep recent events
	keepFrom := n * (100 - keepRecentPercent) / 100
	return a.replaceSession(ctx, sess, finalSummary, keepFrom)
}

// calculateChunkSize calculates adaptive chunk size.
func (a *AdaptiveCompressor) calculateChunkSize(totalTokens, contextWindow int) int {
	// Base chunk size
	baseChunk := int(float64(contextWindow) * BaseChunkRatio)

	// Adjust based on total tokens
	ratio := float64(totalTokens) / float64(contextWindow)
	if ratio > SafetyMargin {
		// Reduce chunk size for large contexts
		minChunk := int(float64(contextWindow) * MinChunkRatio)
		return max(minChunk, baseChunk/2)
	}

	return baseChunk
}

// mergeSummaries merges multiple summaries into one.
func (a *AdaptiveCompressor) mergeSummaries(ctx context.Context, summaries []string) (string, error) {
	if len(summaries) == 1 {
		return summaries[0], nil
	}

	combined := "Merge these segment summaries into one coherent summary. Preserve key decisions, TODOs, unresolved issues, and constraints.\n\n"
	for i, s := range summaries {
		combined += fmt.Sprintf("## Segment %d\n%s\n\n", i+1, s)
	}

	return a.callSummaryLLM(ctx, combined)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
