package bootstrap

import (
	"fmt"
	"log/slog"

	"google.golang.org/adk/model"

	"abot/pkg/agent"
	"abot/pkg/providers"
	"abot/pkg/providers/fallback"
)

// NewProviders creates LLM providers from config.
// Returns (primary, summary, error).
// The first provider is used as primary, and the last provider is used as summary (typically the cheapest).
func NewProviders(cfg *agent.Config) (model.LLM, model.LLM, error) {
	if len(cfg.Providers) == 0 {
		return nil, nil, fmt.Errorf("at least one provider is required")
	}

	// Create all LLM instances.
	entries := make([]fallback.LLMEntry, 0, len(cfg.Providers))
	for i, p := range cfg.Providers {
		llm, modelID, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
			Model:         p.Model,
			APIKey:        p.APIKey,
			APIBase:       p.APIBase,
			PromptCaching: p.PromptCaching,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create LLM [%d] %s: %w", i, p.Name, err)
		}
		entries = append(entries, fallback.LLMEntry{
			Provider: p.Name,
			Model:    modelID,
			LLM:      llm,
		})
	}

	// Wrap in FallbackLLM for automatic failover.
	primary := fallback.NewFallbackLLM(entries, nil)
	slog.Info("providers configured", "count", len(entries))

	// Use last provider as summary LLM (typically the cheapest).
	var summary model.LLM
	if len(entries) > 1 {
		summary = entries[len(entries)-1].LLM
	}

	return primary, summary, nil
}
