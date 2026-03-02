package main

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	"github.com/google/uuid"

	"abot/pkg/agent"
)

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
