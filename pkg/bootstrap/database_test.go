package bootstrap

import (
	"testing"

	"abot/pkg/agent"
)

func TestNewDatabase(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *agent.Config
		wantErr bool
	}{
		{
			name: "no mysql_dsn",
			cfg: &agent.Config{
				MySQLDSN: "",
			},
			wantErr: true,
		},
		{
			name: "invalid mysql_dsn",
			cfg: &agent.Config{
				MySQLDSN: "invalid-dsn",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewDatabase(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && db == nil {
				t.Error("NewDatabase() returned nil db")
			}
		})
	}
}

func TestNewStores(t *testing.T) {
	// This test requires a real database connection, so we skip it in unit tests
	// It should be tested in integration tests
	t.Skip("NewStores requires a real database connection")
}
