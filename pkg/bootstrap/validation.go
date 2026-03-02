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
// Server mode requires providers, mysql_dsn, and optional console settings.
func ValidateForServer(cfg *agent.Config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required for server mode")
	}
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("mysql_dsn is required for server mode")
	}
	// console.addr, console.jwt_secret are optional
	// console.static_dir is not required for server mode
	return nil
}

// ValidateForWeb validates config for web mode.
// Web mode requires providers, mysql_dsn, and console.static_dir.
func ValidateForWeb(cfg *agent.Config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required for web mode")
	}
	if cfg.MySQLDSN == "" {
		return fmt.Errorf("mysql_dsn is required for web mode")
	}
	if cfg.Console.StaticDir == "" {
		return fmt.Errorf("console.static_dir is required for web mode")
	}
	// console.addr, console.jwt_secret are optional
	return nil
}
