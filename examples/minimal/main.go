// Minimal example: Single-file agent with in-memory session
package main

import (
	"context"
	"fmt"
	"log"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
)

func main() {
	// 1. Create minimal configuration
	cfg := &agent.Config{
		AppName: "minimal-bot",
		Providers: []agent.ProviderConfig{
			{
				Name:    "openai",
				APIBase: "https://api.openai.com/v1",
				APIKey:  "sk-your-api-key-here", // Replace with your API key
				Model:   "gpt-4o-mini",
			},
		},
		Agents: []agent.AgentDefConfig{
			{
				ID:          "bot",
				Name:        "assistant",
				Description: "A minimal assistant",
				Model:       "gpt-4o-mini",
			},
		},
		Session: agent.SessionConfig{
			Type: "memory", // In-memory, no persistence
		},
		ContextWindow: 128000,
	}

	// 2. Build lightweight dependencies (no database)
	deps, err := bootstrap.BuildCoreDeps(cfg)
	if err != nil {
		log.Fatalf("build deps: %v", err)
	}

	// 3. Bootstrap agent
	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	fmt.Println("✓ Minimal agent started")
	fmt.Println("Note: This is a minimal example. Use abot-agent for full REPL experience.")

	// 4. Run (blocks until shutdown)
	if err := app.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}
