package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/chzyer/readline"

	"abot/pkg/agent"
	"abot/pkg/types"
)

// historyEntry stores a single conversation turn.
type historyEntry struct {
	User      string
	Assistant string
}

// historyRing is a fixed-size ring buffer for conversation history.
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

func runREPL(ctx context.Context, app *agent.App, tenantID, userID string) error {
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

	// State variables
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

		// Multi-line input detection
		if isMultilineStart(line) {
			full, err := readMultiline(rl, line)
			if err != nil {
				continue
			}
			line = full
		}

		// Slash command dispatch
		if strings.HasPrefix(line, "/") {
			exit := handleSlashCommand(rl, app, line, &activeAgentID, &sessionKey, tenantID, userID, history)
			if exit {
				return nil
			}
			continue
		}

		// Build message
		msg := types.InboundMessage{
			Channel:    "agent",
			TenantID:   tenantID,
			UserID:     userID,
			ChatID:     "direct",
			Content:    line,
			AgentID:    activeAgentID,
			SessionKey: sessionKey,
		}

		// Stream response
		var respBuilder strings.Builder
		streamCtx, streamCancel := context.WithCancel(ctx)

		// Intercept SIGINT for Ctrl+C cancellation
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
				fmt.Fprintf(rl.Stdout(), "\n(interrupted)\n\n")
			} else {
				fmt.Fprintf(rl.Stdout(), "\n[error: %v]\n\n", streamErr)
			}
		}

		// Record in local history
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
func readMultiline(rl *readline.Instance, firstLine string) (string, error) {
	var lines []string

	switch {
	case firstLine == `"""` || strings.HasPrefix(firstLine, `"""`):
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
