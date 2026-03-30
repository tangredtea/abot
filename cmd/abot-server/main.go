package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// 1. Load configuration
	cfg, err := bootstrap.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Validate configuration for server mode
	if err := bootstrap.ValidateForServer(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 3. Build full dependencies (with MySQL)
	result, err := bootstrap.BuildFullDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	// 4. Start core engine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := agent.Bootstrap(ctx, *cfg, *result.Deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// 5. Start background services (cron, heartbeat)
	if err := app.RunServices(ctx); err != nil {
		return fmt.Errorf("run services: %w", err)
	}

	// 6. Start API server
	return runAPIServer(ctx, cancel, cfg, app, result.Deps, result.DB, result.Stores)
}
