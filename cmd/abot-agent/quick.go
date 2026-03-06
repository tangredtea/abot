package main

import (
	"context"
	"fmt"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
)

func runQuick(apiKey, tenantID, userID string, debug bool) error {
	fmt.Println("🚀 Quick start mode")

	// Generate minimal config
	cfg := &agent.Config{
		AppName: "abot",
		Providers: []agent.ProviderConfig{
			{
				Name:    "openai",
				APIBase: "https://api.openai.com/v1",
				APIKey:  apiKey,
				Model:   "gpt-4o-mini",
			},
		},
		Agents: []agent.AgentDefConfig{
			{
				ID:          "default-bot",
				Name:        "assistant",
				Description: "Quick start assistant",
				Model:       "gpt-4o-mini",
			},
		},
		Session: agent.SessionConfig{
			Type: "memory",
		},
		ContextWindow: 128000,
	}

	if debug {
		fmt.Println("🐛 Debug mode enabled")
	}

	// Build deps
	deps, err := bootstrap.BuildCoreDeps(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	// Bootstrap
	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	fmt.Println("✓ Agent started (in-memory session)")

	// Start REPL
	return runREPL(ctx, app, tenantID, userID)
}
