package channels_test

import (
	"context"
	"sync"

	"abot/pkg/types"
)

// mockBus is a minimal MessageBus for testing.
type mockBus struct {
	mu       sync.Mutex
	inbound  []types.InboundMessage
	outbound chan types.OutboundMessage
	closed   bool
}

func newMockBus() *mockBus {
	return &mockBus{
		outbound: make(chan types.OutboundMessage, 16),
	}
}

func (b *mockBus) PublishInbound(_ context.Context, msg types.InboundMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inbound = append(b.inbound, msg)
	return nil
}

func (b *mockBus) ConsumeInbound(_ context.Context) (types.InboundMessage, error) {
	return types.InboundMessage{}, nil
}

func (b *mockBus) PublishOutbound(_ context.Context, msg types.OutboundMessage) error {
	b.outbound <- msg
	return nil
}

func (b *mockBus) ConsumeOutbound(ctx context.Context) (types.OutboundMessage, error) {
	select {
	case <-ctx.Done():
		return types.OutboundMessage{}, ctx.Err()
	case msg := <-b.outbound:
		return msg, nil
	}
}

func (b *mockBus) InboundSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.inbound)
}

func (b *mockBus) OutboundSize() int {
	return len(b.outbound)
}

func (b *mockBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *mockBus) inboundMessages() []types.InboundMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]types.InboundMessage, len(b.inbound))
	copy(cp, b.inbound)
	return cp
}
