package bootstrap

import (
	"testing"

	"abot/pkg/agent"
)

func TestNewProviders(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "single provider",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple providers",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key-1"},
					{Name: "anthropic", Model: "claude-3-haiku", APIKey: "test-key-2"},
				},
			},
			wantErr: false,
		},
		{
			name: "no providers",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{},
			},
			wantErr: true,
		},
		{
			name: "provider with prompt caching",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key", PromptCaching: true},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary, summary, err := NewProviders(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProviders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if primary == nil {
					t.Error("NewProviders() primary is nil")
				}
				// summary can be nil if only one provider
				if len(tt.cfg.Providers) > 1 && summary == nil {
					t.Error("NewProviders() summary is nil with multiple providers")
				}
			}
		})
	}
}
