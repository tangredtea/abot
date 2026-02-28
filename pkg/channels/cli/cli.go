package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"abot/pkg/channels"
	"abot/pkg/types"
)

const (
	ChannelName   = "cli"
	DefaultChatID = "direct"
	DefaultUser   = "user"
)

// CLIChannel is an interactive CLI that reads from stdin and writes to stdout.
type CLIChannel struct {
	*channels.BaseChannel
	reader         *bufio.Reader
	writer         io.Writer
	tenantID       string
	userID         string
	noMarkdown     bool
	statusProvider func() string // optional: returns system status text
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// Option configures a CLIChannel.
type Option func(*CLIChannel)

// WithNoMarkdown disables markdown in output, stripping formatting characters.
func WithNoMarkdown(v bool) Option {
	return func(c *CLIChannel) { c.noMarkdown = v }
}

// WithStatusProvider sets a function that returns system status text for /status.
func WithStatusProvider(fn func() string) Option {
	return func(c *CLIChannel) { c.statusProvider = fn }
}

// NewCLI creates a CLIChannel bound to the given reader/writer and bus.
func NewCLI(bus types.MessageBus, reader io.Reader, writer io.Writer, tenantID, userID string, opts ...Option) *CLIChannel {
	c := &CLIChannel{
		BaseChannel: channels.NewBaseChannel(ChannelName, bus, nil),
		reader:      bufio.NewReader(reader),
		writer:      writer,
		tenantID:    tenantID,
		userID:      userID,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Start begins reading user input from the reader in a goroutine.
func (c *CLIChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}
	c.SetRunning(true)

	rctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.readLoop(rctx)

	return nil
}

// Stop signals the read loop to exit and waits for it to finish.
// For finite readers the goroutine exits on EOF; for blocking readers
// (stdin) it exits once the current ReadString call completes.
func (c *CLIChannel) Stop(_ context.Context) error {
	if !c.IsRunning() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.SetRunning(false)
	return nil
}

// Send writes an outbound message to the writer (stdout).
func (c *CLIChannel) Send(_ context.Context, msg types.OutboundMessage) error {
	text := msg.Content
	if text == "" {
		return nil
	}
	if c.noMarkdown {
		text = stripMarkdown(text)
	}
	_, err := fmt.Fprintln(c.writer, text)
	return err
}

// readLoop reads lines from the reader and publishes them to the bus.
// It runs until the reader returns EOF/error or the channel is stopped.
// For finite readers (e.g. strings.Reader) it drains all input before exiting.
// For blocking readers (e.g. stdin) it blocks on ReadString; Stop() cancels
// the context but the goroutine only exits once the current read completes.
func (c *CLIChannel) readLoop(ctx context.Context) {
	defer c.wg.Done()

	for c.IsRunning() {
		line, err := c.reader.ReadString('\n')
		line = trimLine(line)

		if line != "" {
			c.processLine(ctx, line)
		}

		if err != nil {
			return // EOF or read error
		}
	}
}

// processLine handles a single input line — either a slash command or a message.
func (c *CLIChannel) processLine(ctx context.Context, line string) {
	if resp, handled := c.handleCommand(line); handled {
		if resp != "" {
			fmt.Fprintln(c.writer, resp)
		}
		return
	}
	_ = c.HandleMessage(ctx, c.tenantID, c.userID, DefaultUser, DefaultChatID, line, nil, nil)
}

// handleCommand checks for slash commands. Returns (response, handled).
func (c *CLIChannel) handleCommand(input string) (string, bool) {
	switch {
	case input == "exit" || input == "quit":
		c.SetRunning(false)
		if c.cancel != nil {
			c.cancel()
		}
		return "bye", true
	case input == "/new":
		return "session reset", true
	case input == "/help":
		return helpText(), true
	case input == "/show model":
		return "model: (configured at bootstrap)", true
	case input == "/show channel":
		return fmt.Sprintf("channel: %s", c.Name()), true
	case input == "/status":
		if c.statusProvider != nil {
			return c.statusProvider(), true
		}
		return "status: running", true
	default:
		return "", false
	}
}

func helpText() string {
	return "commands:\n" +
		"  /new          reset session\n" +
		"  /status       show system status\n" +
		"  /help         show this help\n" +
		"  /show model   show current model\n" +
		"  /show channel show current channel\n" +
		"  exit, quit    exit the CLI"
}

func trimLine(s string) string {
	// trim trailing \r\n or \n
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// stripMarkdown removes common markdown formatting for plain-text output.
func stripMarkdown(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lines := strings.Split(s, "\n")
	inCodeBlock := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line)
			continue
		}
		// Strip heading markers.
		cleaned := strings.TrimLeft(line, "# ")
		if strings.TrimSpace(line) != "" && strings.HasPrefix(line, "#") {
			cleaned = strings.TrimLeft(line[strings.IndexByte(line, ' ')+1:], " ")
		} else {
			cleaned = line
		}
		// Strip bold/italic markers.
		cleaned = strings.ReplaceAll(cleaned, "**", "")
		cleaned = strings.ReplaceAll(cleaned, "__", "")
		cleaned = strings.ReplaceAll(cleaned, "*", "")
		cleaned = strings.ReplaceAll(cleaned, "_", "")
		// Strip inline code backticks.
		cleaned = strings.ReplaceAll(cleaned, "`", "")
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(cleaned)
	}
	return b.String()
}
