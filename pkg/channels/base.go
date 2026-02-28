package channels

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"abot/pkg/types"
)

// BaseChannel provides common channel logic that concrete channels can embed.
type BaseChannel struct {
	name      string
	bus       types.MessageBus
	allowList []string // empty = allow all
	running   atomic.Bool
}

// NewBaseChannel creates a BaseChannel with the given name, bus, and optional allowlist.
func NewBaseChannel(name string, bus types.MessageBus, allowList []string) *BaseChannel {
	return &BaseChannel{
		name:      name,
		bus:       bus,
		allowList: allowList,
	}
}

// Name returns the channel name.
func (c *BaseChannel) Name() string { return c.name }

// IsRunning reports whether the channel is currently started.
func (c *BaseChannel) IsRunning() bool { return c.running.Load() }

// SetRunning sets the running state. Concrete channels call this in Start/Stop.
func (c *BaseChannel) SetRunning(v bool) { c.running.Store(v) }

// Bus returns the underlying message bus.
func (c *BaseChannel) Bus() types.MessageBus { return c.bus }

// IsAllowed checks whether senderID is permitted.
// Supports pipe-separated compound IDs (e.g. "123456|username").
// Empty allowlist means allow all.
func (c *BaseChannel) IsAllowed(senderID string) bool {
	if len(c.allowList) == 0 {
		return true
	}
	parts := strings.Split(senderID, "|")
	for _, allowed := range c.allowList {
		for _, part := range parts {
			if strings.EqualFold(strings.TrimSpace(part), strings.TrimSpace(allowed)) {
				return true
			}
		}
	}
	return false
}

// HandleMessage validates the sender and publishes an InboundMessage to the bus.
// Concrete channels call this when they receive user input.
func (c *BaseChannel) HandleMessage(ctx context.Context, tenantID, userID, senderID, chatID, content string, media []string, metadata map[string]string) error {
	if !c.IsAllowed(senderID) {
		return nil // silently drop unauthorized senders
	}
	return c.bus.PublishInbound(ctx, types.InboundMessage{
		Channel:   c.name,
		TenantID:  tenantID,
		UserID:    userID,
		SenderID:  senderID,
		ChatID:    chatID,
		Content:   content,
		Media:     media,
		Metadata:  metadata,
		Timestamp: time.Now(),
	})
}
