package agent_test

import (
	"context"
	"testing"

	"google.golang.org/adk/session"

	"abot/pkg/agent"
)

func TestBootstrap_MinimalConfig(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	llm := &mockLLM{name: "test-model", response: "ok"}

	cfg := agent.Config{
		AppName: "test-app",
		Agents: []agent.AgentDefConfig{
			{ID: "bot-1", Name: "bot-1", Description: "test bot"},
		},
	}

	deps := agent.BootstrapDeps{
		Bus:            mb,
		SessionService: ss,
		LLM:            llm,
	}

	app, err := agent.Bootstrap(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBootstrap_DefaultAppName(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	llm := &mockLLM{name: "m", response: "ok"}

	cfg := agent.Config{
		Agents: []agent.AgentDefConfig{
			{ID: "a", Name: "a", Description: "test"},
		},
	}
	app, err := agent.Bootstrap(context.Background(), cfg, agent.BootstrapDeps{
		Bus: mb, SessionService: ss, LLM: llm,
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBootstrap_WithA2A(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	llm := &mockLLM{name: "m", response: "ok"}

	cfg := agent.Config{
		AppName: "test",
		A2A:     agent.A2AConfig{Enabled: true, Addr: ":0"},
		Agents: []agent.AgentDefConfig{
			{ID: "a", Name: "a", Description: "test"},
		},
	}
	app, err := agent.Bootstrap(context.Background(), cfg, agent.BootstrapDeps{
		Bus: mb, SessionService: ss, LLM: llm,
	})
	if err != nil {
		t.Fatalf("bootstrap with a2a: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBootstrap_WithCompressor(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	llm := &mockLLM{name: "m", response: "ok"}
	summaryLLM := &mockLLM{name: "cheap", response: "summary"}

	cfg := agent.Config{
		AppName:       "test",
		ContextWindow: 64000,
		Agents: []agent.AgentDefConfig{
			{ID: "a", Name: "a", Description: "test"},
		},
	}
	app, err := agent.Bootstrap(context.Background(), cfg, agent.BootstrapDeps{
		Bus: mb, SessionService: ss, LLM: llm, SummaryLLM: summaryLLM,
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app with compressor")
	}
}

func TestShutdown(t *testing.T) {
	mb := newMockBus()
	ss := session.InMemoryService()
	llm := &mockLLM{name: "m", response: "ok"}

	cfg := agent.Config{
		AppName: "test",
		Agents: []agent.AgentDefConfig{
			{ID: "a", Name: "a", Description: "test"},
		},
	}
	app, err := agent.Bootstrap(context.Background(), cfg, agent.BootstrapDeps{
		Bus: mb, SessionService: ss, LLM: llm,
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !mb.closed {
		t.Fatal("expected bus to be closed")
	}
}
