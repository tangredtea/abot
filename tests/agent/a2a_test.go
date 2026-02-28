package agent_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"abot/pkg/agent"
)

func TestSetupA2AServer_Disabled(t *testing.T) {
	srv, err := agent.SetupA2AServer(agent.A2AConfig{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv != nil {
		t.Fatal("expected nil server when disabled")
	}
}

func TestA2AHealthEndpoint(t *testing.T) {
	reg := agent.NewAgentRegistry()
	srv, err := agent.SetupA2AServer(agent.A2AConfig{Enabled: true, Addr: ":0"}, reg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/a2a/health", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestA2AAgentsEndpoint(t *testing.T) {
	reg := agent.NewAgentRegistry()
	reg.Register(newTestEntry("bot-a", nil))
	reg.Register(newTestEntry("bot-b", nil))

	srv, err := agent.SetupA2AServer(agent.A2AConfig{Enabled: true, Addr: ":0"}, reg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/a2a/agents", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "bot-a") || !strings.Contains(body, "bot-b") {
		t.Fatalf("expected both agents in response: %s", body)
	}
}

func TestA2AAgentsEndpoint_Empty(t *testing.T) {
	reg := agent.NewAgentRegistry()
	srv, _ := agent.SetupA2AServer(agent.A2AConfig{Enabled: true, Addr: ":0"}, reg)

	req := httptest.NewRequest(http.MethodGet, "/a2a/agents", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "[]") {
		t.Fatalf("expected empty array, got: %s", w.Body.String())
	}
}

func TestToJSONArray(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, "[]"},
		{[]string{}, "[]"},
		{[]string{"a"}, `["a"]`},
		{[]string{"a", "b"}, `["a","b"]`},
	}
	for _, tt := range tests {
		got := agent.ToJSONArray(tt.in)
		if got != tt.want {
			t.Errorf("ToJSONArray(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
