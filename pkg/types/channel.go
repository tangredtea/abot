package types

import "context"

// Channel is the interface for message transport adapters.
// All concrete channels (CLI, Telegram, Discord, etc.) must implement this interface.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg OutboundMessage) error
	IsAllowed(senderID string) bool
	IsRunning() bool
}
