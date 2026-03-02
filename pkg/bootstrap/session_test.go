package bootstrap

import (
	"testing"

	"abot/pkg/agent"
)

func TestNewSessionService(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "memory session service",
			cfg: &agent.Config{
				Session: agent.SessionConfig{
					Type: "memory",
				},
			},
			wantErr: false,
		},
		{
			name: "default session service (empty type)",
			cfg: &agent.Config{
				Session: agent.SessionConfig{
					Type: "",
				},
			},
			wantErr: false,
		},
		{
			name: "jsonl session service",
			cfg: &agent.Config{
				Session: agent.SessionConfig{
					Type: "jsonl",
					Dir:  t.TempDir(),
				},
			},
			wantErr: false,
		},
		{
			name: "jsonl with default dir",
			cfg: &agent.Config{
				Session: agent.SessionConfig{
					Type: "jsonl",
					Dir:  "",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewSessionService(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSessionService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewSessionService() returned nil service")
			}
		})
	}
}
