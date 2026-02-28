package infra_test

import (
	"testing"

	"abot/pkg/routing"
	"abot/pkg/types"
)

// --- NormalizeAgentID tests ---

func TestNormalizeAgentID_Empty(t *testing.T) {
	if got := routing.NormalizeAgentID(""); got != routing.DefaultAgentID {
		t.Errorf("got %q, want %q", got, routing.DefaultAgentID)
	}
}

func TestNormalizeAgentID_Valid(t *testing.T) {
	if got := routing.NormalizeAgentID("my-agent"); got != "my-agent" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeAgentID_UpperCase(t *testing.T) {
	if got := routing.NormalizeAgentID("My-Agent"); got != "my-agent" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeAgentID_InvalidChars(t *testing.T) {
	got := routing.NormalizeAgentID("hello world!@#")
	if got == "" || got == routing.DefaultAgentID {
		t.Errorf("expected normalized result, got %q", got)
	}
}

func TestNormalizeAgentID_Whitespace(t *testing.T) {
	if got := routing.NormalizeAgentID("  spaces  "); got != "spaces" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeAccountID_Empty(t *testing.T) {
	if got := routing.NormalizeAccountID(""); got != routing.DefaultAccountID {
		t.Errorf("got %q, want %q", got, routing.DefaultAccountID)
	}
}

func TestNormalizeAccountID_Valid(t *testing.T) {
	if got := routing.NormalizeAccountID("acct-1"); got != "acct-1" {
		t.Errorf("got %q", got)
	}
}

// --- Session key tests ---

func TestBuildAgentMainSessionKey(t *testing.T) {
	got := routing.BuildAgentMainSessionKey("bot1")
	if got != "agent:bot1:main" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAgentMainSessionKey_Normalized(t *testing.T) {
	got := routing.BuildAgentMainSessionKey("BOT-1")
	if got != "agent:bot-1:main" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAgentPeerSessionKey_NilPeer(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID: "bot1",
	})
	// nil peer defaults to direct → DMScopeMain → main session key
	if got != "agent:bot1:main" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAgentPeerSessionKey_GroupPeer(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID: "bot1",
		Channel: "discord",
		Peer:    &routing.RoutePeer{Kind: "group", ID: "G123"},
	})
	want := "agent:bot1:discord:group:g123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildAgentPeerSessionKey_DirectPerPeer(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID: "bot1",
		DMScope: routing.DMScopePerPeer,
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "user42"},
	})
	want := "agent:bot1:direct:user42"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildAgentPeerSessionKey_DirectPerChannelPeer(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID: "bot1",
		Channel: "telegram",
		DMScope: routing.DMScopePerChannelPeer,
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "user42"},
	})
	want := "agent:bot1:telegram:direct:user42"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- ParseAgentSessionKey tests ---

func TestParseAgentSessionKey_Valid(t *testing.T) {
	parsed := routing.ParseAgentSessionKey("agent:bot1:main")
	if parsed == nil {
		t.Fatal("expected non-nil")
	}
	if parsed.AgentID != "bot1" || parsed.Rest != "main" {
		t.Errorf("got %+v", parsed)
	}
}

func TestParseAgentSessionKey_Empty(t *testing.T) {
	if routing.ParseAgentSessionKey("") != nil {
		t.Error("expected nil for empty")
	}
}

func TestParseAgentSessionKey_NoPrefix(t *testing.T) {
	if routing.ParseAgentSessionKey("bot1:main") != nil {
		t.Error("expected nil for missing agent: prefix")
	}
}

// --- IsSubagentSessionKey tests ---

func TestIsSubagentSessionKey_True(t *testing.T) {
	if !routing.IsSubagentSessionKey("subagent:task1") {
		t.Error("expected true for subagent: prefix")
	}
}

func TestIsSubagentSessionKey_Nested(t *testing.T) {
	if !routing.IsSubagentSessionKey("agent:bot1:subagent:task1") {
		t.Error("expected true for nested subagent key")
	}
}

func TestIsSubagentSessionKey_False(t *testing.T) {
	if routing.IsSubagentSessionKey("agent:bot1:main") {
		t.Error("expected false for main session key")
	}
}

func TestIsSubagentSessionKey_Empty(t *testing.T) {
	if routing.IsSubagentSessionKey("") {
		t.Error("expected false for empty")
	}
}

// --- RouteResolver tests ---

func TestRouteResolver_DefaultAgent(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{ID: "bot1", IsDefault: true},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{Channel: "cli"})
	if result.AgentID != "bot1" {
		t.Errorf("agent: %q", result.AgentID)
	}
	if result.MatchedBy != "default" {
		t.Errorf("matchedBy: %q", result.MatchedBy)
	}
}

func TestRouteResolver_PeerBinding(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{
				ID: "support-bot",
				Routes: []types.AgentRoute{
					{
						Channel: "telegram",
						Peer:    &types.PeerMatch{Kind: "direct", ID: "vip-user"},
					},
				},
			},
			{ID: "default-bot", IsDefault: true},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{
		Channel: "telegram",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "vip-user"},
	})
	if result.AgentID != "support-bot" {
		t.Errorf("agent: %q, want support-bot", result.AgentID)
	}
	if result.MatchedBy != "binding.peer" {
		t.Errorf("matchedBy: %q", result.MatchedBy)
	}
}

func TestRouteResolver_GuildBinding(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{
				ID: "guild-bot",
				Routes: []types.AgentRoute{
					{Channel: "discord", GuildID: "guild-abc"},
				},
			},
			{ID: "fallback", IsDefault: true},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{
		Channel: "discord",
		GuildID: "guild-abc",
	})
	if result.AgentID != "guild-bot" {
		t.Errorf("agent: %q", result.AgentID)
	}
	if result.MatchedBy != "binding.guild" {
		t.Errorf("matchedBy: %q", result.MatchedBy)
	}
}

func TestRouteResolver_ChannelWildcard(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{
				ID: "catch-all",
				Routes: []types.AgentRoute{
					{Channel: "telegram", AccountID: "*"},
				},
			},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{
		Channel:   "telegram",
		AccountID: "any-account",
	})
	if result.AgentID != "catch-all" {
		t.Errorf("agent: %q", result.AgentID)
	}
	if result.MatchedBy != "binding.channel" {
		t.Errorf("matchedBy: %q", result.MatchedBy)
	}
}

func TestRouteResolver_NoAgents(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{})
	result := resolver.ResolveRoute(routing.RouteInput{Channel: "cli"})
	if result.AgentID != routing.DefaultAgentID {
		t.Errorf("agent: %q, want %q", result.AgentID, routing.DefaultAgentID)
	}
}

func TestRouteResolver_FirstAgentAsDefault(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{ID: "first"},
			{ID: "second"},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{Channel: "cli"})
	if result.AgentID != "first" {
		t.Errorf("agent: %q, want first", result.AgentID)
	}
}

// --- Unique tests from pkg/routing/agent_id_test.go ---

func TestNormalizeAgentID_AllInvalid(t *testing.T) {
	if got := routing.NormalizeAgentID("@@@"); got != routing.DefaultAgentID {
		t.Errorf("NormalizeAgentID('@@@') = %q, want %q", got, routing.DefaultAgentID)
	}
}

func TestNormalizeAgentID_TruncatesAt64(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	got := routing.NormalizeAgentID(long)
	if len(got) > routing.MaxAgentIDLength {
		t.Errorf("length = %d, want <= %d", len(got), routing.MaxAgentIDLength)
	}
}

// --- Unique tests from pkg/routing/route_test.go ---

func TestRouteResolver_TeamBinding(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{ID: "general", IsDefault: true},
			{
				ID: "work",
				Routes: []types.AgentRoute{
					{Channel: "slack", AccountID: "*", TeamID: "T12345"},
				},
			},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{
		Channel: "slack",
		TeamID:  "T12345",
		Peer:    &routing.RoutePeer{Kind: "channel", ID: "C001"},
	})
	if result.AgentID != "work" {
		t.Errorf("AgentID = %q, want 'work'", result.AgentID)
	}
	if result.MatchedBy != "binding.team" {
		t.Errorf("MatchedBy = %q, want 'binding.team'", result.MatchedBy)
	}
}

func TestRouteResolver_PriorityOrder_PeerBeatsGuild(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{ID: "general", IsDefault: true},
			{
				ID: "vip",
				Routes: []types.AgentRoute{
					{
						Channel:   "discord",
						AccountID: "*",
						Peer:      &types.PeerMatch{Kind: "direct", ID: "user-vip"},
					},
				},
			},
			{
				ID: "gaming",
				Routes: []types.AgentRoute{
					{Channel: "discord", AccountID: "*", GuildID: "guild-1"},
				},
			},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{
		Channel: "discord",
		GuildID: "guild-1",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "user-vip"},
	})
	if result.AgentID != "vip" {
		t.Errorf("AgentID = %q, want 'vip' (peer should beat guild)", result.AgentID)
	}
	if result.MatchedBy != "binding.peer" {
		t.Errorf("MatchedBy = %q, want 'binding.peer'", result.MatchedBy)
	}
}

func TestRouteResolver_DefaultAgentSelection(t *testing.T) {
	resolver := routing.NewRouteResolver(routing.RouteConfig{
		Agents: []types.AgentDefinition{
			{ID: "alpha"},
			{ID: "beta", IsDefault: true},
			{ID: "gamma"},
		},
	})
	result := resolver.ResolveRoute(routing.RouteInput{Channel: "cli"})
	if result.AgentID != "beta" {
		t.Errorf("AgentID = %q, want 'beta' (marked as default)", result.AgentID)
	}
}

// --- Unique tests from pkg/routing/session_key_test.go ---

func TestBuildAgentPeerSessionKey_DMScopeMain(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID: "main",
		Channel: "telegram",
		Peer:    &routing.RoutePeer{Kind: "direct", ID: "user123"},
		DMScope: routing.DMScopeMain,
	})
	want := "agent:main:main"
	if got != want {
		t.Errorf("DMScopeMain = %q, want %q", got, want)
	}
}

func TestBuildAgentPeerSessionKey_DMScopePerAccountChannelPeer(t *testing.T) {
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID:   "main",
		Channel:   "telegram",
		AccountID: "bot1",
		Peer:      &routing.RoutePeer{Kind: "direct", ID: "User123"},
		DMScope:   routing.DMScopePerAccountChannelPeer,
	})
	want := "agent:main:telegram:bot1:direct:user123"
	if got != want {
		t.Errorf("DMScopePerAccountChannelPeer = %q, want %q", got, want)
	}
}

func TestBuildAgentPeerSessionKey_IdentityLink(t *testing.T) {
	links := map[string][]string{
		"john": {"telegram:user123", "discord:john#1234"},
	}
	got := routing.BuildAgentPeerSessionKey(routing.SessionKeyParams{
		AgentID:       "main",
		Channel:       "telegram",
		Peer:          &routing.RoutePeer{Kind: "direct", ID: "user123"},
		DMScope:       routing.DMScopePerPeer,
		IdentityLinks: links,
	})
	want := "agent:main:direct:john"
	if got != want {
		t.Errorf("IdentityLink = %q, want %q", got, want)
	}
}
