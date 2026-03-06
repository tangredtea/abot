package agent

import (
	"context"

	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"abot/pkg/types"
)

// testing_exports.go exposes unexported symbols for external (black-box) tests
// in tests/agent/. These wrappers delegate to private methods.

// ExportProcessMessage exposes AgentLoop.processMessage for external tests.
func (al *AgentLoop) ExportProcessMessage(ctx context.Context, msg types.InboundMessage) error {
	return al.processMessage(ctx, msg)
}

// ExportEnsureSession exposes AgentLoop.ensureSession for external tests.
func (al *AgentLoop) ExportEnsureSession(ctx context.Context, msg types.InboundMessage, key string) (session.Session, error) {
	return al.ensureSession(ctx, msg, key)
}

// ExportSafeProcessMessage exposes AgentLoop.safeProcessMessage for external tests.
func (al *AgentLoop) ExportSafeProcessMessage(ctx context.Context, msg types.InboundMessage) error {
	return al.safeProcessMessage(ctx, msg)
}

// ExportRunAgentWithRetry exposes AgentLoop.runAgentWithRetry for external tests.
func (al *AgentLoop) ExportRunAgentWithRetry(ctx context.Context, r *runner.Runner, msg types.InboundMessage, sessionKey string, content *genai.Content) string {
	return al.runAgentWithRetry(ctx, r, msg, sessionKey, content)
}

// ExportBus returns the app's message bus for external tests.
func (a *App) ExportBus() types.MessageBus {
	return a.bus
}

// ExportAgentLoop returns the app's agent loop for external tests.
func (a *App) ExportAgentLoop() *AgentLoop {
	return a.agentLoop
}
