package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/google/uuid"

	"abot/pkg/agent"
	"abot/pkg/bootstrap"
	"abot/pkg/types"
)

func runAgent(cfg *agent.Config) error {
	var deps *agent.BootstrapDeps
	if cfg.MySQLDSN != "" {
		result, err := bootstrap.BuildFullDeps(cfg)
		if err != nil {
			return fmt.Errorf("build deps: %w", err)
		}
		deps = result.Deps
	} else {
		var err error
		deps, err = bootstrap.BuildCoreDeps(cfg)
		if err != nil {
			return fmt.Errorf("build deps: %w", err)
		}
	}
	deps.Channels = nil

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

// historyEntry stores a single conversation turn for /history.
type historyEntry struct {
	User      string
	Assistant string
}

// historyRing is a fixed-size ring buffer for local conversation history.
type historyRing struct {
	entries []historyEntry
	pos     int
	full    bool
	size    int
}

func newHistoryRing(size int) *historyRing {
	return &historyRing{
		entries: make([]historyEntry, size),
		size:    size,
	}
}

func (h *historyRing) push(e historyEntry) {
	h.entries[h.pos] = e
	h.pos = (h.pos + 1) % h.size
	if h.pos == 0 {
		h.full = true
	}
}

func (h *historyRing) list() []historyEntry {
	if !h.full {
		return h.entries[:h.pos]
	}
	result := make([]historyEntry, h.size)
	copy(result, h.entries[h.pos:])
	copy(result[h.size-h.pos:], h.entries[:h.pos])
	return result
}

// buildCompleter constructs the readline tab-completer for slash commands.
func buildCompleter(app *agent.App) *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("/help"),
		readline.PcItem("/agents"),
		readline.PcItem("/switch",
			readline.PcItemDynamic(func(line string) []string {
				return app.ExportRegistry().ListAgents()
			}),
		),
		readline.PcItem("/session",
			readline.PcItem("new"),
			readline.PcItem("info"),
		),
		readline.PcItem("/history"),
		readline.PcItem("/clear"),
		readline.PcItem("/model"),
		readline.PcItem("/status"),
		readline.PcItem("/exit"),
		readline.PcItem("/quit"),
	)
}

func agentREPL(ctx context.Context, app *agent.App, tenantID, userID string) error {
	completer := buildCompleter(app)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "You: ",
		HistoryFile:     "/tmp/abot-history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    completer,
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer rl.Close()

	// State variables.
	activeAgentID := ""
	sessionKey := ""
	history := newHistoryRing(20)

	fmt.Fprintln(rl.Stdout(), "abot agent mode (type /help for commands, exit/quit to leave)")

	for {
		if ctx.Err() != nil {
			return nil
		}

		rl.SetPrompt("You: ")
		line, err := rl.Readline()
		if err == readline.ErrInterrupt || err == io.EOF {
			fmt.Fprintln(rl.Stdout(), "bye")
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			fmt.Fprintln(rl.Stdout(), "bye")
			return nil
		}

		// Multi-line input detection.
		if isMultilineStart(line) {
			full, err := readMultiline(rl, line)
			if err != nil {
				// Ctrl+C during multiline — discard and continue.
				continue
			}
			line = full
		}

		// Slash command dispatch.
		if strings.HasPrefix(line, "/") {
			exit := handleSlashCommand(rl, app, line, &activeAgentID, &sessionKey, tenantID, userID, history)
			if exit {
				return nil
			}
			continue
		}

		// Build the message.
		msg := types.InboundMessage{
			Channel:    "agent",
			TenantID:   tenantID,
			UserID:     userID,
			ChatID:     "direct",
			Content:    line,
			AgentID:    activeAgentID,
			SessionKey: sessionKey,
		}

		// Stream the response.
		var respBuilder strings.Builder
		streamCtx, streamCancel := context.WithCancel(ctx)

		// Intercept SIGINT for Ctrl+C cancellation during streaming.
		streamSig := make(chan os.Signal, 1)
		signal.Notify(streamSig, syscall.SIGINT)
		var sigOnce sync.Once
		go func() {
			select {
			case <-streamSig:
				sigOnce.Do(streamCancel)
			case <-streamCtx.Done():
			}
		}()

		fmt.Fprint(rl.Stdout(), "\n")
		streamErr := app.ExportLoop().ProcessDirectStream(streamCtx, msg, func(ev agent.StreamEvent) {
			switch ev.Type {
			case "text_delta":
				fmt.Fprint(rl.Stdout(), ev.Content)
				respBuilder.WriteString(ev.Content)
			case "tool_call":
				fmt.Fprintf(rl.Stdout(), "\n[calling: %s]\n", ev.Content)
			case "tool_result":
				result := ev.Content
				if len(result) > 200 {
					result = result[:200] + "..."
				}
				fmt.Fprintf(rl.Stdout(), "[result: %s]\n", result)
			case "done":
				fmt.Fprint(rl.Stdout(), "\n\n")
			case "error":
				fmt.Fprintf(rl.Stdout(), "\n[error: %s]\n\n", ev.Error)
			}
		})

		signal.Stop(streamSig)
		sigOnce.Do(streamCancel)

		if streamErr != nil {
			if streamCtx.Err() != nil && ctx.Err() == nil {
				// Cancelled by user Ctrl+C, not a fatal error.
				fmt.Fprintf(rl.Stdout(), "\n(interrupted)\n\n")
			} else {
				slog.Error("process error", "err", streamErr)
				fmt.Fprintf(rl.Stdout(), "\n[error: %v]\n\n", streamErr)
			}
		}

		// Record in local history.
		resp := strings.TrimSpace(respBuilder.String())
		if resp == "" {
			resp = "(no response)"
		}
		history.push(historyEntry{User: line, Assistant: resp})
	}
}

// isMultilineStart checks if a line begins a multiline input block.
func isMultilineStart(line string) bool {
	if line == `"""` || strings.HasPrefix(line, `"""`) {
		return true
	}
	if line == "```" || strings.HasPrefix(line, "```") {
		return true
	}
	if strings.HasSuffix(line, `\`) {
		return true
	}
	return false
}

// readMultiline collects continuation lines until the block is closed.
// Returns readline.ErrInterrupt if cancelled.
func readMultiline(rl *readline.Instance, firstLine string) (string, error) {
	var lines []string

	// Determine mode.
	switch {
	case firstLine == `"""` || strings.HasPrefix(firstLine, `"""`):
		// Triple-quote mode: collect until closing """.
		lines = append(lines, strings.TrimPrefix(firstLine, `"""`))
		rl.SetPrompt("... ")
		for {
			next, err := rl.Readline()
			if err != nil {
				return "", err
			}
			if next == `"""` || strings.HasSuffix(next, `"""`) {
				content := strings.TrimSuffix(next, `"""`)
				if content != "" {
					lines = append(lines, content)
				}
				break
			}
			lines = append(lines, next)
		}

	case firstLine == "```" || strings.HasPrefix(firstLine, "```"):
		// Backtick-fence mode: collect until closing ```.
		lines = append(lines, strings.TrimPrefix(firstLine, "```"))
		rl.SetPrompt("... ")
		for {
			next, err := rl.Readline()
			if err != nil {
				return "", err
			}
			if next == "```" || strings.HasSuffix(next, "```") {
				content := strings.TrimSuffix(next, "```")
				if content != "" {
					lines = append(lines, content)
				}
				break
			}
			lines = append(lines, next)
		}

	case strings.HasSuffix(firstLine, `\`):
		// Backslash continuation: collect until a line doesn't end with \.
		lines = append(lines, strings.TrimSuffix(firstLine, `\`))
		rl.SetPrompt("... ")
		for {
			next, err := rl.Readline()
			if err != nil {
				return "", err
			}
			if strings.HasSuffix(next, `\`) {
				lines = append(lines, strings.TrimSuffix(next, `\`))
			} else {
				lines = append(lines, next)
				break
			}
		}
	}

	return strings.Join(lines, "\n"), nil
}

// handleSlashCommand processes a slash command. Returns true if the REPL should exit.
func handleSlashCommand(rl *readline.Instance, app *agent.App, line string, activeAgentID, sessionKey *string, tenantID, userID string, history *historyRing) bool {
	out := rl.Stdout()
	parts := strings.Fields(line)
	cmd := parts[0]

	switch cmd {
	case "/exit", "/quit":
		fmt.Fprintln(out, "bye")
		return true

	case "/help":
		fmt.Fprintln(out, `Commands:
  /help            Show this help message
  /agents          List available agents
  /switch <id>     Switch to a different agent
  /session new     Start a new session
  /session info    Show current session info
  /history         Show recent conversation history
  /clear           Clear the screen
  /model           Show the current agent's model
  /status          Show system status
  /exit, /quit     Exit the REPL`)

	case "/agents":
		reg := app.ExportRegistry()
		ids := reg.ListAgents()
		if len(ids) == 0 {
			fmt.Fprintln(out, "(no agents registered)")
			break
		}
		fmt.Fprintln(out, "Available agents:")
		for _, id := range ids {
			entry, ok := reg.GetEntry(id)
			if !ok {
				continue
			}
			marker := " "
			if id == *activeAgentID {
				marker = "*"
			}
			name := entry.Config.Name
			if name == "" {
				name = id
			}
			desc := entry.Config.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Fprintf(out, "  %s %-16s %s - %s\n", marker, id, name, desc)
		}

	case "/switch":
		if len(parts) < 2 {
			fmt.Fprintln(out, "Usage: /switch <agent-id>")
			break
		}
		target := parts[1]
		reg := app.ExportRegistry()
		if _, ok := reg.GetEntry(target); !ok {
			fmt.Fprintf(out, "Unknown agent: %s\n", target)
			fmt.Fprintf(out, "Available: %s\n", strings.Join(reg.ListAgents(), ", "))
			break
		}
		*activeAgentID = target
		// Reset session when switching agents.
		*sessionKey = "agent" + ":" + tenantID + ":" + userID + ":" + target + ":" + uuid.New().String()[:8]
		fmt.Fprintf(out, "Switched to agent: %s (new session)\n", target)

	case "/session":
		if len(parts) < 2 {
			fmt.Fprintln(out, "Usage: /session new | /session info")
			break
		}
		switch parts[1] {
		case "new":
			*sessionKey = "agent" + ":" + tenantID + ":" + userID + ":" + uuid.New().String()[:8]
			fmt.Fprintf(out, "New session started: %s\n", *sessionKey)
		case "info":
			sess := *sessionKey
			if sess == "" {
				sess = "(auto: " + tenantID + ":" + userID + ":agent)"
			}
			agentStr := *activeAgentID
			if agentStr == "" {
				agentStr = "(auto-routed)"
			}
			fmt.Fprintf(out, "Tenant:  %s\nUser:    %s\nAgent:   %s\nSession: %s\n", tenantID, userID, agentStr, sess)
		default:
			fmt.Fprintln(out, "Usage: /session new | /session info")
		}

	case "/history":
		entries := history.list()
		if len(entries) == 0 {
			fmt.Fprintln(out, "(no history)")
			break
		}
		for i, e := range entries {
			fmt.Fprintf(out, "[%d] You: %s\n", i+1, truncate(e.User, 80))
			fmt.Fprintf(out, "    Bot: %s\n", truncate(e.Assistant, 80))
		}

	case "/clear":
		fmt.Fprint(out, "\033[H\033[2J")

	case "/model":
		agentID := *activeAgentID
		if agentID == "" {
			// Show first/default agent model.
			ids := app.ExportRegistry().ListAgents()
			if len(ids) > 0 {
				agentID = ids[0]
			}
		}
		if agentID == "" {
			fmt.Fprintln(out, "(no agents registered)")
			break
		}
		entry, ok := app.ExportRegistry().GetEntry(agentID)
		if !ok {
			fmt.Fprintf(out, "Agent not found: %s\n", agentID)
			break
		}
		model := entry.Config.Model
		if model == "" {
			model = "(not set)"
		}
		fmt.Fprintf(out, "Agent: %s\nModel: %s\n", agentID, model)

	case "/status":
		ids := app.ExportRegistry().ListAgents()
		agentStr := *activeAgentID
		if agentStr == "" {
			agentStr = "(auto-routed)"
		}
		sess := *sessionKey
		if sess == "" {
			sess = "(auto)"
		}
		fmt.Fprintf(out, "App:     %s\nAgents:  %d registered\nActive:  %s\nSession: %s\nTenant:  %s\nUser:    %s\n",
			app.AppName(), len(ids), agentStr, sess, tenantID, userID)

	default:
		fmt.Fprintf(out, "Unknown command: %s (type /help for available commands)\n", cmd)
	}

	return false
}

// truncate shortens s to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
