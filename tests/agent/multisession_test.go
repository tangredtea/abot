package agent_test

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"testing"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"abot/pkg/agent"
	"abot/pkg/plugins/memoryconsolidation"
	"abot/pkg/types"
)

// --- scriptedLLM: returns different responses per call ---

type scriptedLLM struct {
	mu        sync.Mutex
	name      string
	responses []string
	callIdx   int
}

func (s *scriptedLLM) Name() string { return s.name }

func (s *scriptedLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	s.mu.Lock()
	idx := s.callIdx
	s.callIdx++
	s.mu.Unlock()

	resp := "default"
	if idx < len(s.responses) {
		resp = s.responses[idx]
	}

	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: resp}},
			},
			TurnComplete: true,
		}, nil)
	}
}

// --- toolCallLLM ---

type toolCallLLM struct {
	mu       sync.Mutex
	name     string
	callIdx  int
	toolName string
	toolArgs map[string]any
	finalMsg string
}

func (tcl *toolCallLLM) Name() string { return tcl.name }

func (tcl *toolCallLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	tcl.mu.Lock()
	idx := tcl.callIdx
	tcl.callIdx++
	tcl.mu.Unlock()

	return func(yield func(*model.LLMResponse, error) bool) {
		if idx == 0 {
			yield(&model.LLMResponse{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{{
						FunctionCall: &genai.FunctionCall{
							Name: tcl.toolName,
							Args: tcl.toolArgs,
						},
					}},
				},
			}, nil)
			return
		}
		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: tcl.finalMsg}},
			},
			TurnComplete: true,
		}, nil)
	}
}

// --- mockVectorStore ---

type mockVectorStore struct {
	mu            sync.Mutex
	collections   map[string]bool
	entries       map[string][]types.VectorEntry
	searches      []mockSearchCall
	searchResults []types.VectorResult
}

type mockSearchCall struct {
	Collection string
	Filter     map[string]any
}

func newMockVectorStore() *mockVectorStore {
	return &mockVectorStore{
		collections: make(map[string]bool),
		entries:     make(map[string][]types.VectorEntry),
	}
}

func (m *mockVectorStore) EnsureCollection(_ context.Context, collection string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collections[collection] = true
	return nil
}

func (m *mockVectorStore) Upsert(_ context.Context, collection string, entries []types.VectorEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[collection] = append(m.entries[collection], entries...)
	return nil
}

func (m *mockVectorStore) Search(_ context.Context, collection string, req *types.VectorSearchRequest) ([]types.VectorResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searches = append(m.searches, mockSearchCall{Collection: collection, Filter: req.Filter})
	if m.searchResults != nil {
		return m.searchResults, nil
	}
	return nil, nil
}

func (m *mockVectorStore) Delete(_ context.Context, _ string, _ map[string]any) error              { return nil }
func (m *mockVectorStore) UpdatePayload(_ context.Context, _ string, _ map[string]any, _ map[string]any) error { return nil }
func (m *mockVectorStore) Close() error                                                            { return nil }

func (m *mockVectorStore) getEntries(collection string) []types.VectorEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]types.VectorEntry, len(m.entries[collection]))
	copy(out, m.entries[collection])
	return out
}

// --- mockEmbedder ---

type mockEmbedder struct{ dim int }

func (e *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, e.dim)
		if len(t) > 0 {
			vec[0] = float32(len(t)) / 100.0
		}
		out[i] = vec
	}
	return out, nil
}

func (e *mockEmbedder) Dimension() int { return e.dim }

// compile-time checks for multisession types
var (
	_ model.LLM        = (*scriptedLLM)(nil)
	_ model.LLM        = (*toolCallLLM)(nil)
	_ types.VectorStore = (*mockVectorStore)(nil)
	_ types.Embedder    = (*mockEmbedder)(nil)
)

// --- helpers ---

func registerScriptedAgent(t *testing.T, reg *agent.AgentRegistry, ss session.Service, appName, agentID string, llm model.LLM, routes []types.AgentRoute) {
	t.Helper()
	a, err := adkagent.New(adkagent.Config{
		Name:        agentID,
		Description: "scripted test agent",
		Run: func(ctx adkagent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				req := &model.LLMRequest{
					Contents: []*genai.Content{
						{Role: "user", Parts: []*genai.Part{{Text: "test"}}},
					},
				}
				for resp, err := range llm.GenerateContent(ctx, req, false) {
					if err != nil {
						yield(nil, err)
						return
					}
					ev := session.NewEvent(ctx.InvocationID())
					ev.Author = agentID
					ev.LLMResponse = model.LLMResponse{
						Content:      resp.Content,
						TurnComplete: resp.TurnComplete,
					}
					yield(ev, nil)
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("create agent %s: %v", agentID, err)
	}
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          a,
		SessionService: ss,
	})
	if err != nil {
		t.Fatalf("create runner %s: %v", agentID, err)
	}
	reg.Register(&agent.AgentEntry{
		ID: agentID, Agent: a, Runner: r,
		Config: types.AgentDefinition{
			ID: agentID, Name: agentID, Routes: routes,
		},
	})
}

// =============================================================================
// Test 1: Multi-turn conversation
// =============================================================================

func TestMultiTurn_ScriptedResponses(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	llm := &scriptedLLM{
		name:      "scripted",
		responses: []string{"reply-1", "reply-2", "reply-3"},
	}

	registerScriptedAgent(t, reg, ss, "test-app", "bot", llm,
		[]types.AgentRoute{{AgentID: "bot", Channel: "cli"}},
	)

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	for i := 0; i < 3; i++ {
		msg := types.InboundMessage{
			TenantID: "t1", UserID: "u1",
			Channel: "cli", ChatID: "c1",
			Content: fmt.Sprintf("turn-%d", i),
		}
		if err := loop.ExportProcessMessage(context.Background(), msg); err != nil {
			t.Fatalf("turn %d: %v", i, err)
		}
	}

	resp, err := ss.Get(context.Background(), &session.GetRequest{
		AppName: "test-app", UserID: "u1", SessionID: agent.SessionKey("t1", "u1", "cli"),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	n := resp.Session.Events().Len()
	if n < 3 {
		t.Fatalf("expected at least 3 events after 3 turns, got %d", n)
	}
	t.Logf("session has %d events after 3 turns", n)

	out := mb.getOutbound()
	if len(out) < 3 {
		t.Fatalf("expected 3 outbound messages, got %d", len(out))
	}
	for i, want := range []string{"reply-1", "reply-2", "reply-3"} {
		if out[i].Content != want {
			t.Errorf("turn %d: expected %q, got %q", i, want, out[i].Content)
		}
	}
}

// =============================================================================
// Test 2: Multiple sessions — tenant isolation
// =============================================================================

func TestMultiSession_TenantIsolation(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	llm := &mockLLM{name: "echo", response: "ok"}
	r, a := newTestRunnerWithLLM(t, ss, "test-app", "shared-bot", llm)

	reg.Register(&agent.AgentEntry{
		ID: "shared-bot", Agent: a, Runner: r,
		Config: types.AgentDefinition{
			ID: "shared-bot", Name: "shared-bot",
			Routes: []types.AgentRoute{{AgentID: "shared-bot", Channel: "*"}},
		},
	})

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)
	ctx := context.Background()

	// Tenant 1 sends 3 messages.
	for i := 0; i < 3; i++ {
		err := loop.ExportProcessMessage(ctx, types.InboundMessage{
			TenantID: "tenant-a", UserID: "alice",
			Channel: "cli", ChatID: "c1",
			Content: fmt.Sprintf("t1-msg-%d", i),
		})
		if err != nil {
			t.Fatalf("tenant-a turn %d: %v", i, err)
		}
	}

	// Tenant 2 sends 1 message.
	err := loop.ExportProcessMessage(ctx, types.InboundMessage{
		TenantID: "tenant-b", UserID: "bob",
		Channel: "cli", ChatID: "c2",
		Content: "t2-msg-0",
	})
	if err != nil {
		t.Fatalf("tenant-b: %v", err)
	}

	// Verify tenant-a session has more events than tenant-b.
	sessA, err := ss.Get(ctx, &session.GetRequest{
		AppName: "test-app", UserID: "alice", SessionID: agent.SessionKey("tenant-a", "alice", "cli"),
	})
	if err != nil {
		t.Fatalf("get tenant-a session: %v", err)
	}
	sessB, err := ss.Get(ctx, &session.GetRequest{
		AppName: "test-app", UserID: "bob", SessionID: agent.SessionKey("tenant-b", "bob", "cli"),
	})
	if err != nil {
		t.Fatalf("get tenant-b session: %v", err)
	}

	nA := sessA.Session.Events().Len()
	nB := sessB.Session.Events().Len()
	t.Logf("tenant-a events: %d, tenant-b events: %d", nA, nB)

	if nA <= nB {
		t.Fatalf("tenant-a (3 turns) should have more events than tenant-b (1 turn): %d vs %d", nA, nB)
	}

	stateA, errA := sessA.Session.State().Get("tenant_id")
	stateB, errB := sessB.Session.State().Get("tenant_id")
	if errA != nil || errB != nil {
		t.Fatalf("state read errors: %v, %v", errA, errB)
	}
	if stateA != "tenant-a" {
		t.Errorf("tenant-a state: expected tenant-a, got %v", stateA)
	}
	if stateB != "tenant-b" {
		t.Errorf("tenant-b state: expected tenant-b, got %v", stateB)
	}
}

// =============================================================================
// Test 3: Agent with tool call
// =============================================================================

func TestToolCall_AgentEmitsFunctionCall(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	a, err := adkagent.New(adkagent.Config{
		Name:        "tool-bot",
		Description: "agent that simulates tool calls",
		Run: func(ctx adkagent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				ev1 := session.NewEvent(ctx.InvocationID())
				ev1.Author = "tool-bot"
				ev1.LLMResponse = model.LLMResponse{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{{
							FunctionCall: &genai.FunctionCall{
								Name: "web_search",
								Args: map[string]any{"query": "golang testing"},
							},
						}},
					},
				}
				if !yield(ev1, nil) {
					return
				}

				ev2 := session.NewEvent(ctx.InvocationID())
				ev2.Author = "tool-bot"
				ev2.Content = &genai.Content{
					Role: "function",
					Parts: []*genai.Part{{
						FunctionResponse: &genai.FunctionResponse{
							Name:     "web_search",
							Response: map[string]any{"results": "Go testing guide"},
						},
					}},
				}
				if !yield(ev2, nil) {
					return
				}

				ev3 := session.NewEvent(ctx.InvocationID())
				ev3.Author = "tool-bot"
				ev3.LLMResponse = model.LLMResponse{
					Content: &genai.Content{
						Role:  "model",
						Parts: []*genai.Part{{Text: "Found: Go testing guide"}},
					},
					TurnComplete: true,
				}
				yield(ev3, nil)
			}
		},
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName: "test-app", Agent: a, SessionService: ss,
	})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	reg.Register(&agent.AgentEntry{
		ID: "tool-bot", Agent: a, Runner: r,
		Config: types.AgentDefinition{
			ID: "tool-bot", Name: "tool-bot",
			Routes: []types.AgentRoute{{AgentID: "tool-bot", Channel: "cli"}},
		},
	})

	loop := agent.NewAgentLoop(mb, reg, ss, nil, "test-app", 128000, nil)

	err = loop.ExportProcessMessage(context.Background(), types.InboundMessage{
		TenantID: "t1", UserID: "u1",
		Channel: "cli", ChatID: "c1",
		Content: "search for golang testing",
	})
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	out := mb.getOutbound()
	if len(out) == 0 {
		t.Fatal("expected outbound message")
	}
	if out[0].Content != "Found: Go testing guide" {
		t.Errorf("expected %q, got %q", "Found: Go testing guide", out[0].Content)
	}

	resp, err := ss.Get(context.Background(), &session.GetRequest{
		AppName: "test-app", UserID: "u1", SessionID: agent.SessionKey("t1", "u1", "cli"),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	n := resp.Session.Events().Len()
	if n < 3 {
		t.Fatalf("expected at least 3 events (tool call round-trip), got %d", n)
	}
	t.Logf("session has %d events after tool call turn", n)
}

// =============================================================================
// Test 4: Memory consolidation
// =============================================================================

// memoryLLM returns a save_memories function call when invoked.
type memoryLLM struct {
	entries []any
}

func (m *memoryLLM) Name() string { return "memory-llm" }

func (m *memoryLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	entries := m.entries
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: "save_memories",
						Args: map[string]any{
							"entries": entries,
						},
					},
				}},
			},
			TurnComplete: true,
		}, nil)
	}
}

func TestMemoryConsolidation_PersistEntries(t *testing.T) {
	vs := newMockVectorStore()
	emb := &mockEmbedder{dim: 8}

	memLLM := &memoryLLM{
		entries: []any{
			map[string]any{"category": "fact", "text": "User likes Go", "scope": "tenant"},
			map[string]any{"category": "preference", "text": "Prefers dark mode", "scope": "user"},
		},
	}

	memPlugin, err := memoryconsolidation.New(memoryconsolidation.Config{
		ConsolidationLLM: memLLM,
		VectorStore:      vs,
		Embedder:         emb,
		MessageThreshold: 1,
	})
	if err != nil {
		t.Fatalf("create memory plugin: %v", err)
	}

	ss := session.InMemoryService()
	mb := newMockBus()
	agentLLM := &mockLLM{name: "agent-llm", response: "got it"}

	cfg := agent.Config{
		AppName:       "mem-test",
		ContextWindow: 128000,
		Agents: []agent.AgentDefConfig{
			{ID: "bot", Name: "bot", Description: "test bot"},
		},
	}
	deps := agent.BootstrapDeps{
		Bus:            mb,
		SessionService: ss,
		LLM:            agentLLM,
		Plugins:        []*plugin.Plugin{memPlugin},
	}

	app, err := agent.Bootstrap(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	loop := app.ExportAgentLoop()
	err = loop.ExportProcessMessage(context.Background(), types.InboundMessage{
		TenantID: "t1", UserID: "u1",
		Channel: "cli", ChatID: "c1",
		Content: "I really like Go programming",
	})
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	entries := vs.getEntries("tenant_t1")
	if len(entries) == 0 {
		t.Fatal("expected entries in vector store after memory consolidation")
	}
	t.Logf("vector store has %d entries in tenant_t1", len(entries))

	for i, e := range entries {
		if e.Payload == nil {
			t.Errorf("entry %d: nil payload", i)
			continue
		}
		if _, ok := e.Payload["text"]; !ok {
			t.Errorf("entry %d: missing text in payload", i)
		}
		if _, ok := e.Payload["category"]; !ok {
			t.Errorf("entry %d: missing category in payload", i)
		}
		if _, ok := e.Payload["scope"]; !ok {
			t.Errorf("entry %d: missing scope in payload", i)
		}
		t.Logf("  entry %d: category=%v scope=%v text=%v",
			i, e.Payload["category"], e.Payload["scope"], e.Payload["text"])
	}
}

// =============================================================================
// Test 5: Memory dedup
// =============================================================================

func TestMemoryConsolidation_Dedup(t *testing.T) {
	vs := newMockVectorStore()
	emb := &mockEmbedder{dim: 8}

	// Pre-configure: Search returns a high-similarity existing entry.
	vs.searchResults = []types.VectorResult{{
		ID:    "old-mem-1",
		Score: 0.95,
		Payload: map[string]any{
			"text":       "User likes Go",
			"category":   "fact",
			"scope":      "tenant",
			"superseded": false,
		},
	}}

	memLLM := &memoryLLM{
		entries: []any{
			map[string]any{
				"category": "fact",
				"text":     "User loves Go and Rust",
				"scope":    "tenant",
			},
		},
	}

	memPlugin, err := memoryconsolidation.New(memoryconsolidation.Config{
		ConsolidationLLM: memLLM,
		VectorStore:      vs,
		Embedder:         emb,
		MessageThreshold: 1,
	})
	if err != nil {
		t.Fatalf("create memory plugin: %v", err)
	}

	ss := session.InMemoryService()
	mb := newMockBus()
	agentLLM := &mockLLM{name: "agent-llm", response: "noted"}

	cfg := agent.Config{
		AppName:       "dedup-test",
		ContextWindow: 128000,
		Agents: []agent.AgentDefConfig{
			{ID: "bot", Name: "bot", Description: "test bot"},
		},
	}
	deps := agent.BootstrapDeps{
		Bus:            mb,
		SessionService: ss,
		LLM:            agentLLM,
		Plugins:        []*plugin.Plugin{memPlugin},
	}

	app, err := agent.Bootstrap(context.Background(), cfg, deps)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	err = app.ExportAgentLoop().ExportProcessMessage(context.Background(), types.InboundMessage{
		TenantID: "t1", UserID: "u1",
		Channel: "cli", ChatID: "c1",
		Content: "I now love Go and Rust",
	})
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	entries := vs.getEntries("tenant_t1")
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 upserts (supersede old + insert new), got %d", len(entries))
	}

	var foundSuperseded, foundNew bool
	for _, e := range entries {
		if e.ID == "old-mem-1" {
			if sup, ok := e.Payload["superseded"].(bool); ok && sup {
				foundSuperseded = true
				t.Logf("old entry marked superseded, superseded_by=%v", e.Payload["superseded_by"])
			}
		} else {
			if sup, ok := e.Payload["superseded"].(bool); ok && !sup {
				foundNew = true
				t.Logf("new entry: id=%s text=%v", e.ID, e.Payload["text"])
			}
		}
	}

	if !foundSuperseded {
		t.Error("old entry was not marked as superseded")
	}
	if !foundNew {
		t.Error("new entry was not inserted")
	}
}
