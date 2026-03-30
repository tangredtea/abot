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
// Supports ${VAR} and $VAR syntax for environment variable substitution.
func LoadConfigWithEnv(path string) (*agent.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expanded := os.ExpandEnv(string(data))

	var cfg agent.Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	return &cfg, nil
}
