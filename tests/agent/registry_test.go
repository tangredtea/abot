package agent_test

import (
	"testing"

	"abot/pkg/agent"
	"abot/pkg/types"
)

func newTestEntry(id string, routes []types.AgentRoute) *agent.AgentEntry {
	return &agent.AgentEntry{
		ID: id,
		Config: types.AgentDefinition{
			ID:     id,
			Name:   id,
			Routes: routes,
		},
	}
}

func TestRegisterAndGetRunner(t *testing.T) {
	reg := agent.NewAgentRegistry()
	entry := newTestEntry("agent-1", nil)
	reg.Register(entry)

	_, ok := reg.GetRunner("agent-1")
	if !ok {
		t.Fatal("expected agent-1 to be registered")
	}

	_, ok = reg.GetRunner("nonexistent")
	if ok {
		t.Fatal("expected nonexistent to not be found")
	}
}

func TestListAgents(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("a", nil))
	reg.Register(newTestEntry("b", nil))

	ids := reg.ListAgents()
	if len(ids) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["a"] || !found["b"] {
		t.Fatalf("missing agents: %v", ids)
	}
}

func TestResolveRoute_ExactMatch(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("general", []types.AgentRoute{
		{AgentID: "general", Channel: "cli"},
	}))
	reg.Register(newTestEntry("vip", []types.AgentRoute{
		{AgentID: "vip", Channel: "cli", ChatID: "chat-42"},
	}))

	got := reg.ResolveRoute(types.InboundMessage{Channel: "cli", ChatID: "chat-42"})
	if got != "vip" {
		t.Fatalf("expected vip, got %s", got)
	}
}

func TestResolveRoute_ChannelOnly(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("discord-bot", []types.AgentRoute{
		{AgentID: "discord-bot", Channel: "discord"},
	}))

	got := reg.ResolveRoute(types.InboundMessage{Channel: "discord", ChatID: "any"})
	if got != "discord-bot" {
		t.Fatalf("expected discord-bot, got %s", got)
	}
}

func TestResolveRoute_Fallback(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("default", nil))

	got := reg.ResolveRoute(types.InboundMessage{Channel: "unknown"})
	if got != "default" {
		t.Fatalf("expected default, got %s", got)
	}
}

func TestResolveRoute_NoAgents(t *testing.T) {
	reg := agent.NewAgentRegistry()
	got := reg.ResolveRoute(types.InboundMessage{Channel: "cli"})
	if got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

func TestGetEntry(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("bot-1", nil))

	entry, ok := reg.GetEntry("bot-1")
	if !ok {
		t.Fatal("expected bot-1 entry to exist")
	}
	if entry.ID != "bot-1" {
		t.Fatalf("expected ID bot-1, got %s", entry.ID)
	}
	if entry.Config.Name != "bot-1" {
		t.Fatalf("expected config name bot-1, got %s", entry.Config.Name)
	}

	_, ok = reg.GetEntry("nonexistent")
	if ok {
		t.Fatal("expected nonexistent entry to not be found")
	}
}

func TestGetDefaultAgent(t *testing.T) {
	reg := agent.NewAgentRegistry()

	if got := reg.GetDefaultAgent(); got != nil {
		t.Fatalf("expected nil for empty registry, got %v", got)
	}

	reg.Register(newTestEntry("first", nil))
	reg.Register(newTestEntry("second", nil))

	got := reg.GetDefaultAgent()
	if got == nil {
		t.Fatal("expected non-nil default agent")
	}
	if got.ID != "first" && got.ID != "second" {
		t.Fatalf("unexpected default agent ID: %s", got.ID)
	}
}

func TestMatchScore(t *testing.T) {
	tests := []struct {
		name  string
		route types.AgentRoute
		msg   types.InboundMessage
		want  int
	}{
		{"wildcard", types.AgentRoute{}, types.InboundMessage{Channel: "cli"}, 0},
		{"channel match", types.AgentRoute{Channel: "cli"}, types.InboundMessage{Channel: "cli"}, 1},
		{"channel mismatch", types.AgentRoute{Channel: "discord"}, types.InboundMessage{Channel: "cli"}, -1},
		{"channel+chatID match", types.AgentRoute{Channel: "cli", ChatID: "c1"}, types.InboundMessage{Channel: "cli", ChatID: "c1"}, 2},
		{"chatID mismatch", types.AgentRoute{Channel: "cli", ChatID: "c1"}, types.InboundMessage{Channel: "cli", ChatID: "c2"}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agent.MatchScore(&tt.route, tt.msg)
			if got != tt.want {
				t.Errorf("MatchScore = %d, want %d", got, tt.want)
			}
		})
	}
}
