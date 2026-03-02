package bootstrap

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"abot/pkg/agent"
)

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*agent.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg agent.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	return &cfg, nil
}

// LoadConfigWithEnv loads config and expands environment variables.
// This function is reserved for future use when environment variable expansion is needed.
func LoadConfigWithEnv(path string) (*agent.Config, error) {
	// For now, just call LoadConfig.
	// TODO: Add environment variable expansion support (e.g., ${OPENAI_API_KEY}).
	return LoadConfig(path)
}
