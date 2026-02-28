package memoryconsolidation

import (
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"

	"abot/pkg/types"
)

// Config for the memory consolidation plugin.
type Config struct {
	ConsolidationLLM model.LLM
	VectorStore      types.VectorStore
	Embedder         types.Embedder
	MemoryEventStore types.MemoryEventStore // optional, for writing event log
	MessageThreshold int                    // default 50
	DedupScore       float32                // similarity threshold for dedup (default 0.85)
}

// New creates a memory consolidation plugin.
func New(cfg Config) (*plugin.Plugin, error) {
	if cfg.ConsolidationLLM == nil {
		return nil, fmt.Errorf("memoryconsolidation: ConsolidationLLM is required")
	}
	if cfg.VectorStore == nil {
		return nil, fmt.Errorf("memoryconsolidation: VectorStore is required")
	}
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("memoryconsolidation: Embedder is required")
	}
	threshold := cfg.MessageThreshold
	if threshold <= 0 {
		threshold = 50
	}
	dedupScore := cfg.DedupScore
	if dedupScore <= 0 {
		dedupScore = defaultDedupScore
	}
	c := &consolidator{
		llm:              cfg.ConsolidationLLM,
		vectorStore:      cfg.VectorStore,
		embedder:         cfg.Embedder,
		memoryEventStore: cfg.MemoryEventStore,
		threshold:        threshold,
		dedupScore:       dedupScore,
	}
	return plugin.New(plugin.Config{
		Name:             "memoryconsolidation",
		AfterRunCallback: c.afterRun,
	})
}
