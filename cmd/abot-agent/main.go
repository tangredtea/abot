package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
	"abot/pkg/types"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	tenantID := flag.String("tenant", types.DefaultTenantID, "tenant ID")
	userID := flag.String("user", types.DefaultUserID, "user ID")
	flag.Parse()

	if err := run(*configPath, *tenantID, *userID); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath, tenantID, userID string) error {
	// 1. Load configuration
	cfg, err := bootstrap.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Validate configuration for agent mode
	if err := bootstrap.ValidateForAgent(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 3. Auto-create default agent if no agents configured
	if len(cfg.Agents) == 0 {
		cfg.Agents = []agent.AgentDefConfig{
			{
				ID:    "default-bot",
				Name:  "assistant",
				Model: cfg.Providers[0].Model,
			},
		}
	}

	// 4. Build lightweight dependencies (no MySQL)
	deps, err := bootstrap.BuildCoreDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	// 5. Bootstrap the agent engine
	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// 6. Start REPL
	return runREPL(ctx, app, tenantID, userID)
}
