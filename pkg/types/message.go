// Package types defines shared interfaces and types for the ABot agent framework.
// All cross-package dependencies go through this package to ensure isolation.
package types

import (
	"context"
	"time"
)

// InboundMessage represents a message arriving from any channel.
type InboundMessage struct {
	Channel    string
	TenantID   string
	UserID     string
	SenderID   string
	ChatID     string
	Content    string
	Media      []string
	Metadata   map[string]string
	SessionKey string // Optional override; defaults to auto-derived by EffectiveSessionKey.
	AgentID    string // Optional override; bypasses route resolution when set.
	Timestamp  time.Time
}

// EffectiveSessionKey returns the session identifier.
// If SessionKey is set, it is returned directly; otherwise it is derived as "{tenant_id}:{user_id}:{channel}".
func (m *InboundMessage) EffectiveSessionKey() string {
	if m.SessionKey != "" {
		return m.SessionKey
	}
	return m.TenantID + ":" + m.UserID + ":" + m.Channel
}

// OutboundMessage represents a message to be sent to a channel.
type OutboundMessage struct {
	Channel  string
	ChatID   string
	Content  string
	Media    []string
	Metadata map[string]string
}

// MessageBus decouples channels/scheduler from agent core.
type MessageBus interface {
	PublishInbound(ctx context.Context, msg InboundMessage) error
	ConsumeInbound(ctx context.Context) (InboundMessage, error)
	PublishOutbound(ctx context.Context, msg OutboundMessage) error
	ConsumeOutbound(ctx context.Context) (OutboundMessage, error)
	// InboundSize returns the number of pending messages in the inbound queue (for monitoring and backpressure).
	InboundSize() int
	// OutboundSize returns the number of pending messages in the outbound queue.
	OutboundSize() int
	Close() error
}
