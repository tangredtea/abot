package agent_test

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/session"

	"abot/pkg/agent"
	"abot/pkg/bus"
	"abot/pkg/channels/cli"
	"abot/pkg/types"
)

type integrationHarness struct {
	app    *agent.App
	ss     session.Service
	inW    *io.PipeWriter
	outR   *bufio.Reader
	outPR  *io.PipeReader
	cancel context.CancelFunc
	errCh  chan error
}

type harnessOpts struct {
	response      string
	summaryLLM    *mockLLM
	contextWindow int
}

func newIntegrationHarness(t *testing.T, response string) *integrationHarness {
	t.Helper()
	return newIntegrationHarnessWithOpts(t, harnessOpts{response: response})
}

func newIntegrationHarnessWithOpts(t *testing.T, opts harnessOpts) *integrationHarness {
	t.Helper()

	inR, inW := io.Pipe()
	outPR, outW := io.Pipe()

	msgBus := bus.New(10)
	ss := session.InMemoryService()
	llm := &mockLLM{name: "integration-model", response: opts.response}
	cliCh := cli.NewCLI(msgBus, inR, outW, "default", "user")

	ctxWindow := opts.contextWindow
	if ctxWindow <= 0 {
		ctxWindow = 128000
	}

	cfg := agent.Config{
		AppName:       "integration-test",
		ContextWindow: ctxWindow,
		Agents: []agent.AgentDefConfig{
			{ID: "bot", Name: "bot", Description: "integration test bot"},
		},
	}
	deps := agent.BootstrapDeps{
		Bus:            msgBus,
		SessionService: ss,
		LLM:            llm,
		Channels:       map[string]types.Channel{cli.ChannelName: cliCh},
	}
	if opts.summaryLLM != nil {
		deps.SummaryLLM = opts.summaryLLM
	}

	app, err := agent.Bootstrap(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	return &integrationHarness{
		app:   app,
		ss:    ss,
		inW:   inW,
		outR:  bufio.NewReader(outPR),
		outPR: outPR,
		errCh: make(chan error, 1),
	}
}

func (h *integrationHarness) start(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.errCh <- h.app.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
}

func (h *integrationHarness) shutdown(t *testing.T) {
	t.Helper()
	h.inW.Close()
	h.cancel()
	select {
	case err := <-h.errCh:
		if err != nil {
			t.Fatalf("app.Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("app.Run did not exit after shutdown")
	}
}

func (h *integrationHarness) send(t *testing.T, msg string) {
	t.Helper()
	if _, err := io.WriteString(h.inW, msg+"\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}
}

func (h *integrationHarness) recv(t *testing.T, timeout time.Duration) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		line, _ := h.outR.ReadString('\n')
		done <- strings.TrimSpace(line)
	}()
	select {
	case line := <-done:
		return line
	case <-time.After(timeout):
		t.Fatal("timeout waiting for output")
		return ""
	}
}

func TestIntegration_CLIEndToEnd(t *testing.T) {
	h := newIntegrationHarness(t, "hello from bot")
	h.start(t)

	h.send(t, "hi there")
	got := h.recv(t, 5*time.Second)
	if got != "hello from bot" {
		t.Fatalf("expected %q, got %q", "hello from bot", got)
	}

	h.shutdown(t)
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	h := newIntegrationHarness(t, "ok")
	h.start(t)

	// Shut down immediately without sending messages.
	h.shutdown(t)

	// Verify bus is closed.
	err := h.app.ExportBus().PublishInbound(context.Background(), types.InboundMessage{})
	if err == nil {
		t.Fatal("expected bus to be closed after shutdown")
	}
}

func TestIntegration_MultiTurn(t *testing.T) {
	h := newIntegrationHarness(t, "echo reply")
	h.start(t)

	for i := range 2 {
		h.send(t, "turn")
		got := h.recv(t, 5*time.Second)
		if got != "echo reply" {
			t.Fatalf("turn %d: expected %q, got %q", i, "echo reply", got)
		}
	}

	h.shutdown(t)
}

// getSession fetches the session created by CLI channel messages.
func (h *integrationHarness) getSession(t *testing.T) session.Session {
	t.Helper()
	resp, err := h.ss.Get(context.Background(), &session.GetRequest{
		AppName:   "integration-test",
		UserID:    "user",
		SessionID: agent.SessionKey("default", "user", "cli", "bot"),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	return resp.Session
}

func TestIntegration_SessionAccumulation(t *testing.T) {
	h := newIntegrationHarness(t, "reply")
	h.start(t)

	for i := range 3 {
		h.send(t, "msg")
		got := h.recv(t, 5*time.Second)
		if got != "reply" {
			t.Fatalf("turn %d: expected %q, got %q", i, "reply", got)
		}
	}

	sess := h.getSession(t)
	n := sess.Events().Len()
	if n < 3 {
		t.Fatalf("expected session to accumulate events across turns, got %d", n)
	}
	t.Logf("session has %d events after 3 turns", n)

	h.shutdown(t)
}

func TestIntegration_CompressionTrigger(t *testing.T) {
	h := newIntegrationHarnessWithOpts(t, harnessOpts{
		response:      "compressed bot reply",
		summaryLLM:    &mockLLM{name: "summary-model", response: "conversation summary"},
		contextWindow: 1,
	})
	h.start(t)

	for i := range 3 {
		h.send(t, "hello")
		got := h.recv(t, 5*time.Second)
		if got != "compressed bot reply" {
			t.Fatalf("turn %d: expected %q, got %q", i, "compressed bot reply", got)
		}
	}

	sess := h.getSession(t)
	n := sess.Events().Len()
	t.Logf("session has %d events after compression", n)

	if n >= 6 {
		t.Fatalf("expected compression to reduce events below 6, got %d", n)
	}

	h.shutdown(t)
}
