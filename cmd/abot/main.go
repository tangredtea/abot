package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
	"abot/pkg/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Detect subcommand before flag.Parse().
	subcmd := ""
	if len(os.Args) > 1 && (os.Args[1] == "agent" || os.Args[1] == "console") {
		subcmd = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...) // strip subcommand for flag parsing
	}

	configPath := flag.String("config", "config.yaml", "path to config file")
	tenantFlag := flag.String("tenant", types.DefaultTenantID, "tenant ID for CLI/agent mode")
	userFlag := flag.String("user", types.DefaultUserID, "user ID for CLI/agent mode")
	flag.Parse()

	cfg, err := bootstrap.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	cfg.CLITenantID = *tenantFlag
	cfg.CLIUserID = *userFlag

	if subcmd == "agent" {
		return runAgent(cfg)
	}
	if subcmd == "console" {
		return runConsole(cfg)
	}

	return runGateway(cfg)
}

func runGateway(cfg *agent.Config) error {
	result, err := bootstrap.BuildFullDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *result.Deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	slog.Info("starting gateway", "app", cfg.AppName)
	return app.Run(ctx)
}
