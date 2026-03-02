package bootstrap

import (
	"testing"

	"abot/pkg/agent"
)

func TestBuildCoreDeps(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
				},
				Session: agent.SessionConfig{
					Type: "memory",
				},
				ObjectStore: agent.ObjectStoreConfig{
					Dir: t.TempDir(),
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
			name: "with MCP config",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
				},
				MCP: map[string]agent.MCPServerConfig{
					"test-server": {
						Command: "test-command",
						Args:    []string{"arg1", "arg2"},
					},
				},
			},
			wantErr: true, // MCP connection will fail without real server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := BuildCoreDeps(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildCoreDeps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if deps == nil {
					t.Error("BuildCoreDeps() returned nil deps")
					return
				}
				if deps.LLM == nil {
					t.Error("BuildCoreDeps() LLM is nil")
				}
				if deps.SessionService == nil {
					t.Error("BuildCoreDeps() SessionService is nil")
				}
				if deps.Bus == nil {
					t.Error("BuildCoreDeps() Bus is nil")
				}
				if deps.Tools == nil {
					t.Error("BuildCoreDeps() Tools is nil")
				}
			}
		})
	}
}

func TestBuildFullDeps(t *testing.T) {
	// This test requires a real database connection, so we only test error cases
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "no mysql_dsn",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
				},
				MySQLDSN: "",
			},
			wantErr: true,
		},
		{
			name: "invalid mysql_dsn",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test-key"},
				},
				MySQLDSN: "invalid-dsn",
			},
			wantErr: true,
		},
		{
			name: "no providers",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{},
				MySQLDSN:  "user:pass@tcp(localhost:3306)/db",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildFullDeps(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFullDeps() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
