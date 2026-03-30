package bootstrap

import (
	"fmt"

	"abot/pkg/agent"
)

// ValidateForAgent validates config for agent mode.
// Agent mode requires minimal configuration: providers and optional agents.
func ValidateForAgent(cfg *agent.Config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required for agent mode")
	}
	// mysql_dsn and console are optional for agent mode
	return nil
}

// ValidateForServer validates config for server mode.
// Server mode requires providers, mysql_dsn, and jwt_secret.
func ValidateForServer(cfg *agent.Config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required for server mode")
	}
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("mysql_dsn is required for server mode")
	}
	if cfg.Console.JWTSecret == "" {
		return fmt.Errorf("console.jwt_secret is required for server mode (do not use default secrets in production)")
	}
	return nil
}

// ValidateForWeb validates config for web mode.
// Web mode requires providers, mysql_dsn, and jwt_secret.
func ValidateForWeb(cfg *agent.Config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required for web mode")
	}
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("mysql_dsn is required for web mode")
	}
	if cfg.Console.JWTSecret == "" {
		return fmt.Errorf("console.jwt_secret is required for web mode (do not use default secrets in production)")
	}
	return nil
}
