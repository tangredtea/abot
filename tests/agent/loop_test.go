package agent_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/session"

	"abot/pkg/agent"
	"abot/pkg/types"
)

func TestEnsureSession_CreateNew(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	msg := types.InboundMessage{
		TenantID: "t1",
		UserID:   "u1",
		Channel:  "cli",
		ChatID:   "c1",
	}

	key := agent.SessionKey("t1", "u1", "cli")
	sess, err := loop.ExportEnsureSession(context.Background(), msg, key)
	if err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if sess.ID() != key {
		t.Fatalf("expected session ID %s, got %s", key, sess.ID())
	}
}

func TestEnsureSession_ExistingSession(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()
	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	ctx := context.Background()
	msg := types.InboundMessage{TenantID: "t1", UserID: "u1", Channel: "cli"}

	key := agent.SessionKey("t1", "u1", "cli")
	// Create first.
	sess1, err := loop.ExportEnsureSession(ctx, msg, key)
	if err != nil {
		t.Fatalf("first ensureSession: %v", err)
	}

	// Second call should return existing.
	sess2, err := loop.ExportEnsureSession(ctx, msg, key)
	if err != nil {
		t.Fatalf("second ensureSession: %v", err)
	}
	if sess1.ID() != sess2.ID() {
		t.Fatalf("expected same session, got %s vs %s", sess1.ID(), sess2.ID())
	}
}

func TestProcessMessage_FullFlow(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	r, a := newTestRunner(t, ss, "test-app", "echo-bot", "hello back")

	reg.Register(&agent.AgentEntry{
		ID:     "echo-bot",
		Agent:  a,
		Runner: r,
		Config: types.AgentDefinition{
			ID:   "echo-bot",
			Name: "echo-bot",
			Routes: []types.AgentRoute{
				{AgentID: "echo-bot", Channel: "cli"},
			},
		},
	})

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	msg := types.InboundMessage{
		TenantID: "t1",
		UserID:   "u1",
		Channel:  "cli",
		ChatID:   "c1",
		Content:  "hi there",
	}

	if err := loop.ExportProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	out := mb.getOutbound()
	if len(out) == 0 {
		t.Fatal("expected outbound message")
	}
	if out[0].Channel != "cli" || out[0].ChatID != "c1" {
		t.Fatalf("unexpected outbound routing: %+v", out[0])
	}
}

func TestProcessMessage_NoAgent(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()
	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	msg := types.InboundMessage{Channel: "unknown", UserID: "u1"}
	err := loop.ExportProcessMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for no matching agent")
	}
}

func TestIsContextOverflowError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil error", nil, false},
		{"token limit", fmt.Errorf("request exceeds token limit"), true},
		{"context_length_exceeded", fmt.Errorf("context_length_exceeded"), true},
		{"max_tokens", fmt.Errorf("max_tokens exceeded"), true},
		{"maximum context length", fmt.Errorf("maximum context length is 128000"), true},
		{"context window", fmt.Errorf("context window exceeded"), true},
		{"too many tokens", fmt.Errorf("too many tokens in request"), true},
		{"request too large", fmt.Errorf("request too large"), true},
		{"prompt is too long", fmt.Errorf("prompt is too long"), true},
		{"unrelated error", fmt.Errorf("network timeout"), false},
		{"permission denied", fmt.Errorf("permission denied"), false},
		{"generic context word", fmt.Errorf("context cancelled"), false},
		{"generic token word", fmt.Errorf("invalid token"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agent.IsContextOverflowError(tt.err)
			if got != tt.expect {
				t.Fatalf("IsContextOverflowError(%v) = %v, want %v", tt.err, got, tt.expect)
			}
		})
	}
}

func TestSafeProcessMessage_PanicRecovery(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()
	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	msg := types.InboundMessage{Channel: "cli", UserID: "u1"}
	err := loop.ExportSafeProcessMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error from safeProcessMessage")
	}
}

func TestRunAgentWithRetry_NonRetryableError(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	errLLM := &mockLLM{name: "err-model", err: fmt.Errorf("network timeout")}
	r, a := newTestRunnerWithLLM(t, ss, "test-app", "err-bot", errLLM)

	reg.Register(&agent.AgentEntry{
		ID:     "err-bot",
		Agent:  a,
		Runner: r,
		Config: types.AgentDefinition{
			ID:   "err-bot",
			Name: "err-bot",
			Routes: []types.AgentRoute{
				{AgentID: "err-bot", Channel: "cli"},
			},
		},
	})

	comp := agent.NewCompressor(&mockLLM{response: "summary"}, ss, "test-app")
	loop := agent.NewAgentLoop(mb, reg, ss, comp, "test-app", 128000, nil)

	msg := types.InboundMessage{
		TenantID: "t1", UserID: "u1",
		Channel: "cli", ChatID: "c1",
		Content: "hello",
	}

	_, err := loop.ExportEnsureSession(context.Background(), msg, agent.SessionKey("t1", "u1", "cli"))
	if err != nil {
		t.Fatalf("ensureSession: %v", err)
	}

	content := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: "hello"}},
	}

	result := loop.ExportRunAgentWithRetry(context.Background(), r, msg, agent.SessionKey("t1", "u1", "cli"), content)
	if result != "" {
		t.Fatalf("expected empty result for non-retryable error, got %q", result)
	}
}

func TestRunAgentWithRetry_NoCompressor(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	errLLM := &mockLLM{name: "overflow-model", err: fmt.Errorf("token limit exceeded")}
	r, a := newTestRunnerWithLLM(t, ss, "test-app", "overflow-bot", errLLM)

	reg.Register(&agent.AgentEntry{
		ID:     "overflow-bot",
		Agent:  a,
		Runner: r,
		Config: types.AgentDefinition{
			ID:   "overflow-bot",
			Name: "overflow-bot",
			Routes: []types.AgentRoute{
				{AgentID: "overflow-bot", Channel: "cli"},
			},
		},
	})

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	msg := types.InboundMessage{
		TenantID: "t1", UserID: "u1",
		Channel: "cli", ChatID: "c1",
		Content: "hello",
	}

	_, err := loop.ExportEnsureSession(context.Background(), msg, agent.SessionKey("t1", "u1", "cli"))
	if err != nil {
		t.Fatalf("ensureSession: %v", err)
	}

	content := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: "hello"}},
	}

	result := loop.ExportRunAgentWithRetry(context.Background(), r, msg, agent.SessionKey("t1", "u1", "cli"), content)
	if result != "" {
		t.Fatalf("expected empty result without compressor, got %q", result)
	}
}
