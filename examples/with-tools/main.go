// Example: Agent with built-in tools (web search, file operations)
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
		AppName: "tool-bot",
		Providers: []agent.ProviderConfig{
			{
				Name:    "openai",
				APIBase: "https://api.openai.com/v1",
				APIKey:  "sk-your-api-key-here",
				Model:   "gpt-4o-mini",
			},
		},
		Agents: []agent.AgentDefConfig{
			{
				ID:          "tool-bot",
				Name:        "assistant",
				Description: "Assistant with tools",
				Model:       "gpt-4o-mini",
			},
		},
		Session: agent.SessionConfig{
			Type: "jsonl",
			Dir:  "data/sessions",
		},
		Sandbox: agent.SandboxConfig{
			Level:               "standard",
			RestrictToWorkspace: true,
			ExecMemoryMB:        512,
			ExecCPUSeconds:      30,
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

	fmt.Println("✓ Agent with tools started")
	fmt.Println("Available tools: web_search, read_file, write_file, shell_exec")

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}
