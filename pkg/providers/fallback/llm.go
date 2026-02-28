package fallback

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"google.golang.org/adk/model"
)

// LLMEntry pairs a model.LLM with its provider name for fallback tracking.
type LLMEntry struct {
	Provider string
	Model    string
	LLM      model.LLM
}

// FallbackLLM implements model.LLM with automatic failover across providers.
type FallbackLLM struct {
	entries  []LLMEntry
	chain    *Chain
}

// NewFallbackLLM creates a FallbackLLM from multiple provider entries.
// If only one entry is provided, GenerateContent delegates directly (no overhead).
func NewFallbackLLM(entries []LLMEntry, cooldown *CooldownTracker) *FallbackLLM {
	if cooldown == nil {
		cooldown = NewCooldownTracker()
	}
	return &FallbackLLM{
		entries: entries,
		chain:   NewChain(cooldown),
	}
}

// Name returns a composite name of all providers.
func (f *FallbackLLM) Name() string {
	names := make([]string, len(f.entries))
	for i, e := range f.entries {
		names[i] = fmt.Sprintf("%s/%s", e.Provider, e.Model)
	}
	return "fallback[" + strings.Join(names, ",") + "]"
}

// GenerateContent implements model.LLM with fallback across providers.
func (f *FallbackLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	// Fast path: single provider, no overhead.
	if len(f.entries) == 1 {
		return f.entries[0].LLM.GenerateContent(ctx, req, stream)
	}

	return func(yield func(*model.LLMResponse, error) bool) {
		f.generateWithFallback(ctx, req, stream, yield)
	}
}

// generateWithFallback tries each provider in order, falling back on retriable errors.
func (f *FallbackLLM) generateWithFallback(
	ctx context.Context,
	req *model.LLMRequest,
	stream bool,
	yield func(*model.LLMResponse, error) bool,
) {
	candidates := make([]Candidate, len(f.entries))
	for i, e := range f.entries {
		candidates[i] = Candidate{Provider: e.Provider, Model: e.Model}
	}

	// Build a lookup map for LLM instances.
	llmByProvider := make(map[string]model.LLM, len(f.entries))
	for _, e := range f.entries {
		llmByProvider[e.Provider] = e.LLM
	}

	var lastResp *model.LLMResponse
	var lastErr error

	_, chainErr := f.chain.Execute(ctx, candidates, func(ctx context.Context, provider, modelName string) error {
		llm := llmByProvider[provider]
		slog.Debug("fallback: trying provider", "provider", provider, "model", modelName)

		for resp, err := range llm.GenerateContent(ctx, req, stream) {
			if err != nil {
				lastErr = err
				return err
			}
			lastResp = resp
			lastErr = nil
		}
		return nil
	})

	if chainErr != nil {
		// All providers failed — yield the last error.
		if lastErr == nil {
			lastErr = chainErr
		}
		slog.Warn("fallback: all providers failed", "err", chainErr)
		yield(nil, lastErr)
		return
	}

	// Success — yield the final response.
	if lastResp != nil {
		yield(lastResp, nil)
	}
}
