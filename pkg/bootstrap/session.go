package bootstrap

import (
	"fmt"
	"log/slog"

	"google.golang.org/adk/session"

	"abot/pkg/agent"
	abotsession "abot/pkg/session"
)

// NewSessionService creates a session service based on config.
// Supports "jsonl" and "memory" types.
func NewSessionService(cfg *agent.Config) (session.Service, error) {
	switch cfg.Session.Type {
	case "jsonl":
		dir := cfg.Session.Dir
		if dir == "" {
			dir = "data/sessions"
		}
		svc, err := abotsession.NewJSONLService(dir)
		if err != nil {
			return nil, fmt.Errorf("jsonl session: %w", err)
		}
		slog.Info("session service configured", "type", "jsonl", "dir", dir)
		return svc, nil
	default:
		slog.Info("session service configured", "type", "in-memory")
		return session.InMemoryService(), nil
	}
}
