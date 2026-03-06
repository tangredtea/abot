// Package steering provides message steering and interruption capabilities.
//
// Inspired by OpenClaw's steering mechanism:
// - Check for new messages after each tool execution
// - Interrupt remaining tools when steering detected
// - Inject steering messages into next turn
package steering

import (
	"context"
	"sync"

	"abot/pkg/types"
)

// Manager handles steering message detection and interruption.
type Manager struct {
	bus           types.MessageBus
	pendingBySession map[string][]types.InboundMessage
	mu            sync.RWMutex
}

// NewManager creates a steering manager.
func NewManager(bus types.MessageBus) *Manager {
	return &Manager{
		bus:           bus,
		pendingBySession: make(map[string][]types.InboundMessage),
	}
}

// CheckSteering checks if there are new messages for the session.
// Returns pending messages that should interrupt current execution.
func (m *Manager) CheckSteering(ctx context.Context, sessionKey string) ([]types.InboundMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we have pending messages
	if msgs, ok := m.pendingBySession[sessionKey]; ok && len(msgs) > 0 {
		// Return and clear
		m.pendingBySession[sessionKey] = nil
		return msgs, nil
	}

	// Try to peek from bus (non-blocking)
	select {
	case msg := <-m.peekInbound(sessionKey):
		return []types.InboundMessage{msg}, nil
	default:
		return nil, nil
	}
}

// QueueSteering queues a message for steering.
func (m *Manager) QueueSteering(sessionKey string, msg types.InboundMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pendingBySession[sessionKey] = append(m.pendingBySession[sessionKey], msg)
}

// peekInbound attempts to peek a message from bus without blocking.
func (m *Manager) peekInbound(sessionKey string) <-chan types.InboundMessage {
	ch := make(chan types.InboundMessage, 1)
	go func() {
		// This is a simplified implementation
		// In production, you'd need to filter by sessionKey
		close(ch)
	}()
	return ch
}
