// Example: Multi-agent system with fallback providers
package main

import (
	"context"
	"fmt"
	"log"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
)

func main() {
	cfg := &agent.Config{
		AppName: "multi-agent",
		// Multiple providers with automatic failover
		Providers: []agent.ProviderConfig{
			{
				Name:    "primary",
				APIBase: "https://api.openai.com/v1",
				APIKey:  "sk-your-openai-key",
				Model:   "gpt-4o",
			},
			{
				Name:    "fallback",
				APIBase: "https://api.openai.com/v1",
				APIKey:  "sk-your-openai-key",
				Model:   "gpt-4o-mini",
			},
		},
		// Multiple agents with different roles
		Agents: []agent.AgentDefConfig{
			{
				ID:          "coder",
				Name:        "coder",
				Description: "Expert programmer",
				Model:       "gpt-4o",
			},
			{
				ID:          "reviewer",
				Name:        "reviewer",
				Description: "Code reviewer",
				Model:       "gpt-4o-mini",
			},
		},
		Session: agent.SessionConfig{
			Type: "jsonl",
			Dir:  "data/sessions",
		},
		ContextWindow: 128000,
	}

	deps, err := bootstrap.BuildCoreDeps(cfg)
	if err != nil {
		log.Fatalf("build deps: %v", err)
	}

	ctx := context.Background()
	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	fmt.Println("✓ Multi-agent system started")
	fmt.Println("Agents: coder (gpt-4o), reviewer (gpt-4o-mini)")
	fmt.Println("Providers: primary (gpt-4o) → fallback (gpt-4o-mini)")

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}
