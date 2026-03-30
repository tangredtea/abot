// Package steering provides ADK-based steering interruption.
//
// Based on Google ADK interruption mechanism:
// - ctx.end_invocation = True for graceful stop
// - Client-side termination for streaming
package steering

import (
	"context"
	"sync"

	"abot/pkg/types"
)

// ADKSteering provides ADK-compatible steering interruption.
type ADKSteering struct {
	bus              types.MessageBus
	activeContexts   map[string]context.CancelFunc
	pendingMessages  map[string][]types.InboundMessage
	mu               sync.RWMutex
}

// NewADKSteering creates an ADK steering manager.
func NewADKSteering(bus types.MessageBus) *ADKSteering {
	return &ADKSteering{
		bus:             bus,
		activeContexts:  make(map[string]context.CancelFunc),
		pendingMessages: make(map[string][]types.InboundMessage),
	}
}

// RegisterContext registers an active agent context for potential interruption.
func (s *ADKSteering) RegisterContext(sessionKey string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeContexts[sessionKey] = cancel
}

// UnregisterContext removes an agent context after completion.
func (s *ADKSteering) UnregisterContext(sessionKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeContexts, sessionKey)
}

// InterruptIfNeeded checks for steering messages and interrupts if found.
// Returns true if interrupted, false otherwise.
func (s *ADKSteering) InterruptIfNeeded(sessionKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for pending messages
	if msgs, ok := s.pendingMessages[sessionKey]; ok && len(msgs) > 0 {
		// Interrupt active context
		if cancel, exists := s.activeContexts[sessionKey]; exists {
			cancel() // Trigger ctx.Done()
			return true
		}
	}

	return false
}

// QueueSteeringMessage queues a message for steering interruption.
func (s *ADKSteering) QueueSteeringMessage(sessionKey string, msg types.InboundMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingMessages[sessionKey] = append(s.pendingMessages[sessionKey], msg)

	// Trigger interruption if context is active
	if cancel, exists := s.activeContexts[sessionKey]; exists {
		cancel()
	}
}

// GetPendingMessages retrieves and clears pending steering messages.
func (s *ADKSteering) GetPendingMessages(sessionKey string) []types.InboundMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := s.pendingMessages[sessionKey]
	s.pendingMessages[sessionKey] = nil
	return msgs
}
