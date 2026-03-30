package bootstrap

import (
	"testing"

	"abot/pkg/agent"
)

func TestValidateForAgent(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "valid agent config",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
			},
			wantErr: false,
		},
		{
			name: "agent config without mysql_dsn is valid",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForAgent(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateForAgent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateForServer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "valid server config",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "user:pass@tcp(localhost:3306)/db",
				Console:  agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: false,
		},
		{
			name: "no providers",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{},
				MySQLDSN:  "user:pass@tcp(localhost:3306)/db",
				Console:   agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: true,
		},
		{
			name: "no mysql_dsn",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "",
				Console:  agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: true,
		},
		{
			name: "no jwt_secret",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "user:pass@tcp(localhost:3306)/db",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForServer(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateForServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateForWeb(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "valid web config",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "user:pass@tcp(localhost:3306)/db",
				Console:  agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: false,
		},
		{
			name: "no providers",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{},
				MySQLDSN:  "user:pass@tcp(localhost:3306)/db",
				Console:   agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: true,
		},
		{
			name: "no mysql_dsn",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "",
				Console:  agent.ConsoleConfig{JWTSecret: "my-secret"},
			},
			wantErr: true,
		},
		{
			name: "no jwt_secret",
			cfg: &agent.Config{
				Providers: []agent.ProviderConfig{
					{Name: "openai", Model: "gpt-4o-mini", APIKey: "test"},
				},
				MySQLDSN: "user:pass@tcp(localhost:3306)/db",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateForWeb(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateForWeb() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
