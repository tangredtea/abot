package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := `
app_name: test-app
providers:
  - name: openai
    model: gpt-4o-mini
    api_key: test-key
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid config",
			path:    configPath,
			wantErr: false,
		},
		{
			name:    "file not found",
			path:    filepath.Join(tmpDir, "notfound.yaml"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cfg == nil {
				t.Error("LoadConfig() returned nil config")
			}
			if !tt.wantErr && cfg.AppName != "test-app" {
				t.Errorf("LoadConfig() app_name = %v, want test-app", cfg.AppName)
			}
		})
	}
}

func TestLoadConfigWithEnv(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := `
app_name: test-app
providers:
  - name: openai
    model: gpt-4o-mini
    api_key: test-key
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfigWithEnv(configPath)
	if err != nil {
		t.Fatalf("LoadConfigWithEnv() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfigWithEnv() returned nil config")
	}
	if cfg.AppName != "test-app" {
		t.Errorf("LoadConfigWithEnv() app_name = %v, want test-app", cfg.AppName)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidConfig := `
app_name: test
providers:
  - name: openai
    invalid yaml here [[[
`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() expected error for invalid YAML, got nil")
	}
}
