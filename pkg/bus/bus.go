// Package bus implements an in-process message bus using buffered Go channels.
// It decouples channel/scheduler producers from the agent core consumer.
package bus

import (
	"context"
	"errors"
	"sync/atomic"

	"abot/pkg/types"
)

// ErrBusClosed is returned when operating on a closed bus.
var ErrBusClosed = errors.New("bus: closed")

// DefaultBufferSize is the default channel buffer capacity.
const DefaultBufferSize = 100

// ChannelBus implements types.MessageBus using buffered Go channels.
// Safe for concurrent use by multiple goroutines.
type ChannelBus struct {
	inbound  chan types.InboundMessage
	outbound chan types.OutboundMessage
	closed   atomic.Bool
	done     chan struct{} // signals Close() to unblock selects
}

// New creates a ChannelBus with the given buffer size.
// If bufferSize <= 0, DefaultBufferSize is used.
func New(bufferSize int) *ChannelBus {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	return &ChannelBus{
		inbound:  make(chan types.InboundMessage, bufferSize),
		outbound: make(chan types.OutboundMessage, bufferSize),
		done:     make(chan struct{}),
	}
}

// PublishInbound sends an inbound message to the bus.
// Blocks if the buffer is full until ctx is cancelled or the bus is closed.
func (b *ChannelBus) PublishInbound(ctx context.Context, msg types.InboundMessage) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	select {
	case b.inbound <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.done:
		return ErrBusClosed
	}
}

// ConsumeInbound blocks until an inbound message is available,
// the context is cancelled, or the bus is closed.
func (b *ChannelBus) ConsumeInbound(ctx context.Context) (types.InboundMessage, error) {
	select {
	case msg, ok := <-b.inbound:
		if !ok {
			return types.InboundMessage{}, ErrBusClosed
		}
		return msg, nil
	case <-ctx.Done():
		return types.InboundMessage{}, ctx.Err()
	case <-b.done:
		// Drain any remaining messages before returning closed.
		select {
		case msg, ok := <-b.inbound:
			if ok {
				return msg, nil
			}
		default:
		}
		return types.InboundMessage{}, ErrBusClosed
	}
}

// PublishOutbound sends an outbound message to the bus.
// Blocks if the buffer is full until ctx is cancelled or the bus is closed.
func (b *ChannelBus) PublishOutbound(ctx context.Context, msg types.OutboundMessage) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	select {
	case b.outbound <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.done:
		return ErrBusClosed
	}
}

// ConsumeOutbound blocks until an outbound message is available,
// the context is cancelled, or the bus is closed.
func (b *ChannelBus) ConsumeOutbound(ctx context.Context) (types.OutboundMessage, error) {
	select {
	case msg, ok := <-b.outbound:
		if !ok {
			return types.OutboundMessage{}, ErrBusClosed
		}
		return msg, nil
	case <-ctx.Done():
		return types.OutboundMessage{}, ctx.Err()
	case <-b.done:
		select {
		case msg, ok := <-b.outbound:
			if ok {
				return msg, nil
			}
		default:
		}
		return types.OutboundMessage{}, ErrBusClosed
	}
}

// InboundSize returns the number of pending messages in the inbound queue.
func (b *ChannelBus) InboundSize() int {
	return len(b.inbound)
}

// OutboundSize returns the number of pending messages in the outbound queue.
func (b *ChannelBus) OutboundSize() int {
	return len(b.outbound)
}

// Close shuts down the bus. All blocked Consume/Publish calls return immediately.
// Safe to call multiple times; only the first call has effect.
// Data channels are NOT closed to avoid send-on-closed-channel panics;
// the done channel is sufficient to unblock all select statements.
func (b *ChannelBus) Close() error {
	if b.closed.Swap(true) {
		return nil // already closed
	}
	close(b.done)
	return nil
}
