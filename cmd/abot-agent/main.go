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
	// Check for init subcommand
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInitWizard(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	configPath := flag.String("config", "config.yaml", "path to config file")
	tenantID := flag.String("tenant", types.DefaultTenantID, "tenant ID")
	userID := flag.String("user", types.DefaultUserID, "user ID")
	debug := flag.Bool("debug", false, "enable debug mode")
	quick := flag.Bool("quick", false, "quick start mode (no config file)")
	apiKey := flag.String("api-key", "", "OpenAI API key (for quick mode)")
	flag.Parse()

	// Quick mode: generate config on-the-fly
	if *quick {
		if *apiKey == "" {
			*apiKey = os.Getenv("OPENAI_API_KEY")
			if *apiKey == "" {
				fmt.Fprintf(os.Stderr, "error: --api-key required in quick mode (or set OPENAI_API_KEY)\n")
				os.Exit(1)
			}
		}
		if err := runQuick(*apiKey, *tenantID, *userID, *debug); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*configPath, *tenantID, *userID, *debug); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath, tenantID, userID string, debug bool) error {
	if debug {
		fmt.Println("🐛 Debug mode enabled")
	}
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
