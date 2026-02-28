// Package providers contains the provider registry and LLM implementations.
package providers

import (
	"strings"
	"sync"

	"abot/pkg/types"
)

// Registry holds all known provider specs and routes model names to providers.
type Registry struct {
	mu    sync.RWMutex
	specs []types.ProviderSpec
	byName map[string]*types.ProviderSpec
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]*types.ProviderSpec),
	}
}

// Register adds a provider spec. Order matters for match priority.
func (r *Registry) Register(spec types.ProviderSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs = append(r.specs, spec)
	r.byName[spec.Name] = &r.specs[len(r.specs)-1]
}

// FindByName returns a provider spec by its canonical name.
func (r *Registry) FindByName(name string) *types.ProviderSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[name]
}

// FindByModel matches a standard provider by model-name keyword.
// Skips gateways — those are matched by api_key/api_base instead.
func (r *Registry) FindByModel(model string) *types.ProviderSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(model)
	prefix := ""
	if idx := strings.Index(lower, "/"); idx > 0 {
		prefix = lower[:idx]
	}

	// Phase 1: explicit provider prefix match (e.g. "anthropic/claude-3").
	for i := range r.specs {
		s := &r.specs[i]
		if s.IsGateway {
			continue
		}
		if prefix != "" && prefix == s.Name {
			return s
		}
	}

	// Phase 2: keyword match (word-boundary-aware for short keywords).
	for i := range r.specs {
		s := &r.specs[i]
		if s.IsGateway {
			continue
		}
		for _, kw := range s.Keywords {
			if ContainsKeyword(lower, kw) {
				return s
			}
		}
	}

	return nil
}

// FindGateway detects a gateway/local provider by name, key prefix, or base URL keyword.
func (r *Registry) FindGateway(providerName, apiKey, apiBase string) *types.ProviderSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct match by name.
	if providerName != "" {
		if s := r.byName[providerName]; s != nil && s.IsGateway {
			return s
		}
	}

	keyLower := strings.ToLower(apiKey)
	baseLower := strings.ToLower(apiBase)

	for i := range r.specs {
		s := &r.specs[i]
		if !s.IsGateway {
			continue
		}
		if s.EnvKey != "" && keyLower != "" && strings.HasPrefix(keyLower, strings.ToLower(s.Name)) {
			return s
		}
		if s.DefaultAPIBase != "" && baseLower != "" && strings.Contains(baseLower, strings.ToLower(s.Name)) {
			return s
		}
	}

	return nil
}

// All returns all registered specs in priority order.
func (r *Registry) All() []types.ProviderSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]types.ProviderSpec, len(r.specs))
	copy(out, r.specs)
	return out
}

// ContainsKeyword checks if s contains kw. For short keywords (≤2 chars),
// it enforces word boundaries so "o1" won't match inside "pro1xy".
func ContainsKeyword(s, kw string) bool {
	if len(kw) > 2 {
		return strings.Contains(s, kw)
	}
	idx := 0
	for {
		pos := strings.Index(s[idx:], kw)
		if pos < 0 {
			return false
		}
		pos += idx
		leftOK := pos == 0 || !isAlnum(s[pos-1])
		end := pos + len(kw)
		rightOK := end == len(s) || !isAlnum(s[end])
		if leftOK && rightOK {
			return true
		}
		idx = pos + 1
	}
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// ModelRef represents a parsed "provider/model" reference.
type ModelRef struct {
	Provider string
	Model    string
}

// ParseModelRef parses "anthropic/claude-opus" into {Provider: "anthropic", Model: "claude-opus"}.
// If no slash, uses defaultProvider.
func ParseModelRef(raw string, defaultProvider string) *ModelRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if idx := strings.Index(raw, "/"); idx > 0 {
		provider := NormalizeProvider(raw[:idx])
		model := strings.TrimSpace(raw[idx+1:])
		if model == "" {
			return nil
		}
		return &ModelRef{Provider: provider, Model: model}
	}
	return &ModelRef{
		Provider: NormalizeProvider(defaultProvider),
		Model:    raw,
	}
}

// NormalizeProvider normalizes provider identifiers to canonical form.
func NormalizeProvider(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "gpt":
		return "openai"
	case "claude":
		return "anthropic"
	case "google":
		return "gemini"
	}
	return p
}

// ModelKey returns a canonical "provider/model" key for deduplication.
func ModelKey(provider, model string) string {
	return NormalizeProvider(provider) + "/" + strings.ToLower(strings.TrimSpace(model))
}

// DefaultRegistry returns a registry pre-populated with common providers.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register(types.ProviderSpec{
		Name:           "anthropic",
		Keywords:       []string{"anthropic", "claude"},
		DefaultAPIBase: "https://api.anthropic.com",
		EnvKey:         "ANTHROPIC_API_KEY",
	})

	r.Register(types.ProviderSpec{
		Name:           "openai",
		Keywords:       []string{"openai", "gpt", "o1", "o3", "o4"},
		DefaultAPIBase: "https://api.openai.com/v1",
		EnvKey:         "OPENAI_API_KEY",
	})

	r.Register(types.ProviderSpec{
		Name:           "deepseek",
		Keywords:       []string{"deepseek"},
		DefaultAPIBase: "https://api.deepseek.com/v1",
		EnvKey:         "DEEPSEEK_API_KEY",
	})

	r.Register(types.ProviderSpec{
		Name:           "gemini",
		Keywords:       []string{"gemini"},
		DefaultAPIBase: "https://generativelanguage.googleapis.com",
		EnvKey:         "GEMINI_API_KEY",
	})

	r.Register(types.ProviderSpec{
		Name:           "openrouter",
		Keywords:       []string{"openrouter"},
		IsGateway:      true,
		DefaultAPIBase: "https://openrouter.ai/api/v1",
		EnvKey:         "OPENROUTER_API_KEY",
	})

	return r
}
