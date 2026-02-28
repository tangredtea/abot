package integration_test

import (
	"bufio"
	"context"
	"io"
	"iter"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"

	"abot/pkg/agent"
	"abot/pkg/bus"
	"abot/pkg/channels/cli"
	"abot/pkg/types"
)

// mockLLM implements model.LLM for integration testing.
type mockLLM struct {
	name     string
	response string
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: m.response}},
			},
		}, nil)
	}
}

var _ model.LLM = (*mockLLM)(nil)

// harness wires up the full app for end-to-end testing via CLI channel.
type harness struct {
	app    *agent.App
	ss     session.Service
	inW    *io.PipeWriter
	outR   *bufio.Reader
	outPR  *io.PipeReader
	cancel context.CancelFunc
	errCh  chan error
}

func newHarness(t *testing.T, response string) *harness {
	t.Helper()

	inR, inW := io.Pipe()
	outPR, outW := io.Pipe()

	msgBus := bus.New(10)
	ss := session.InMemoryService()
	llm := &mockLLM{name: "test-model", response: response}
	cliCh := cli.NewCLI(msgBus, inR, outW, "default", "user")

	cfg := agent.Config{
		AppName: "integration-test",
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

	app, err := agent.Bootstrap(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	return &harness{
		app:   app,
		ss:    ss,
		inW:   inW,
		outR:  bufio.NewReader(outPR),
		outPR: outPR,
		errCh: make(chan error, 1),
	}
}

func (h *harness) start(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.errCh <- h.app.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
}

func (h *harness) shutdown(t *testing.T) {
	t.Helper()
	h.inW.Close()
	h.cancel()
	select {
	case err := <-h.errCh:
		if err != nil {
			t.Fatalf("app.Run error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("app.Run did not exit")
	}
}

func (h *harness) send(t *testing.T, msg string) {
	t.Helper()
	if _, err := io.WriteString(h.inW, msg+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func (h *harness) recv(t *testing.T, timeout time.Duration) string {
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

// --- Integration tests ---

func TestIntegration_EndToEnd(t *testing.T) {
	h := newHarness(t, "bot says hello")
	h.start(t)

	h.send(t, "hi")
	got := h.recv(t, 5*time.Second)
	if got != "bot says hello" {
		t.Fatalf("expected %q, got %q", "bot says hello", got)
	}

	h.shutdown(t)
}

func TestIntegration_MultiTurn(t *testing.T) {
	h := newHarness(t, "echo")
	h.start(t)

	for i := range 3 {
		h.send(t, "ping")
		got := h.recv(t, 5*time.Second)
		if got != "echo" {
			t.Fatalf("turn %d: expected %q, got %q", i, "echo", got)
		}
	}

	h.shutdown(t)
}
