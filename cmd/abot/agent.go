package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/chzyer/readline"

	"abot/pkg/agent"
	"abot/pkg/types"
)

// buildDepsForAgent builds all dependencies but with no channels.
// Agent mode drives the agent loop directly via ProcessDirect.
func buildDepsForAgent(cfg *agent.Config) (*agent.BootstrapDeps, error) {
	deps, err := buildDeps(cfg)
	if err != nil {
		return nil, err
	}
	deps.Channels = nil
	return deps, nil
}

func runAgent(cfg *agent.Config) error {
	deps, err := buildDepsForAgent(cfg)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := agent.Bootstrap(ctx, *cfg, *deps)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// Start background services (cron + heartbeat) only.
	if err := app.RunServices(ctx); err != nil {
		return fmt.Errorf("run services: %w", err)
	}

	// Handle signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	tenantID := cfg.CLITenantID
	if tenantID == "" {
		tenantID = types.DefaultTenantID
	}
	userID := cfg.CLIUserID
	if userID == "" {
		userID = types.DefaultUserID
	}

	return agentREPL(ctx, app, tenantID, userID)
}

func agentREPL(ctx context.Context, app *agent.App, tenantID, userID string) error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "You: ",
		HistoryFile:     "/tmp/abot-history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer rl.Close()

	fmt.Println("abot agent mode (type exit/quit to leave)")

	for {
		if ctx.Err() != nil {
			return nil
		}

		line, err := rl.Readline()
		if err == readline.ErrInterrupt || err == io.EOF {
			fmt.Println("bye")
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline: %w", err)
		}

		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			fmt.Println("bye")
			return nil
		}

		msg := types.InboundMessage{
			Channel:  "agent",
			TenantID: tenantID,
			UserID:   userID,
			ChatID:   "direct",
			Content:  line,
		}

		resp, err := app.ProcessDirect(ctx, msg)
		if err != nil {
			slog.Error("process error", "err", err)
			fmt.Printf("\n⚠ %v\n\n", err)
			continue
		}

		if resp != "" {
			fmt.Printf("\n🤖 %s\n\n", resp)
		} else {
			fmt.Print("\n🤖 (no response)\n\n")
		}
	}
}
