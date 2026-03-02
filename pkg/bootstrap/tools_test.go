package bootstrap

import (
	"testing"

	"abot/pkg/agent"
	"abot/pkg/bus"
)

func TestBuildLightweightTools(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "basic config",
			cfg: &agent.Config{
				ObjectStore: agent.ObjectStoreConfig{
					Dir: t.TempDir(),
				},
			},
			wantErr: false,
		},
		{
			name: "with sandbox config",
			cfg: &agent.Config{
				ObjectStore: agent.ObjectStoreConfig{
					Dir: t.TempDir(),
				},
				Sandbox: agent.SandboxConfig{
					ExecMemoryMB:        512,
					ExecCPUSeconds:      10,
					ExecFileSizeMB:      100,
					ExecNProc:           50,
					Level:               "strict",
					RestrictToWorkspace: true,
					RateLimit:           10,
					RateBurst:           20,
				},
			},
			wantErr: false,
		},
		{
			name: "with default object store dir",
			cfg: &agent.Config{
				ObjectStore: agent.ObjectStoreConfig{
					Dir: "",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := BuildLightweightTools(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildLightweightTools() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tools == nil {
				t.Error("BuildLightweightTools() returned nil tools")
			}
		})
	}
}

func TestBuildFullTools(t *testing.T) {
	// This test requires database stores, so we test with minimal setup
	msgBus := bus.New(100)
	stores := &StoreBundle{
		// All stores are nil, which is acceptable for this test
	}

	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "basic config",
			cfg: &agent.Config{
				ObjectStore: agent.ObjectStoreConfig{
					Dir: t.TempDir(),
				},
			},
			wantErr: false,
		},
		{
			name: "with sandbox config",
			cfg: &agent.Config{
				ObjectStore: agent.ObjectStoreConfig{
					Dir: t.TempDir(),
				},
				Sandbox: agent.SandboxConfig{
					ExecMemoryMB:        512,
					ExecCPUSeconds:      10,
					Level:               "strict",
					RestrictToWorkspace: true,
					RateLimit:           10,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := BuildFullTools(tt.cfg, stores, msgBus)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFullTools() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tools == nil {
				t.Error("BuildFullTools() returned nil tools")
			}
		})
	}
}
