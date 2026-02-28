package agent

import (
	"fmt"
	"net/http"
	"strings"
)

// A2AConfig holds A2A server settings.
type A2AConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// SetupA2AServer creates an HTTP server exposing agents via A2A protocol.
// NOTE: Full A2A implementation requires github.com/a2aproject/a2a-go dependency.
// Once available, this will use adka2a.NewExecutor + a2asrv to serve agents.
// For now, it exposes a health endpoint and agent listing.
func SetupA2AServer(cfg A2AConfig, registry *AgentRegistry) (*http.Server, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("/a2a/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// List available agents.
	mux.HandleFunc("/a2a/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := registry.ListAgents()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"agents":%v}`, ToJSONArray(agents))
	})

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}, nil
}

func ToJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
