package channels

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"abot/pkg/types"
)

// ChannelAdapter bridges a Channel to the MessageBus.
// Outbound dispatch is handled centrally by the Registry, not per-adapter.
type ChannelAdapter struct {
	channel types.Channel
	bus     types.MessageBus
}

// NewAdapter creates a ChannelAdapter for the given channel and bus.
func NewAdapter(ch types.Channel, bus types.MessageBus) *ChannelAdapter {
	return &ChannelAdapter{
		channel: ch,
		bus:     bus,
	}
}

// Channel returns the underlying channel.
func (a *ChannelAdapter) Channel() types.Channel { return a.channel }

// Start launches the channel (outbound dispatch is handled by Registry).
func (a *ChannelAdapter) Start(ctx context.Context) error {
	if err := a.channel.Start(ctx); err != nil {
		return fmt.Errorf("channels: start %s: %w", a.channel.Name(), err)
	}
	return nil
}

// Stop stops the channel.
func (a *ChannelAdapter) Stop(ctx context.Context) error {
	return a.channel.Stop(ctx)
}

// Send delivers an outbound message via the channel.
func (a *ChannelAdapter) Send(ctx context.Context, msg types.OutboundMessage) error {
	if err := a.channel.Send(ctx, msg); err != nil {
		slog.Error("channels: send", "channel", a.channel.Name(), "err", err)
		return err
	}
	return nil
}

// OutboundDispatcher consumes outbound messages from the bus and routes
// them to the correct channel adapter. Only one goroutine runs this,
// avoiding the competing-consumer bug of per-adapter dispatch.
type OutboundDispatcher struct {
	bus      types.MessageBus
	adapters func(name string) (*ChannelAdapter, bool) // lookup function
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewOutboundDispatcher creates a dispatcher that routes outbound messages.
func NewOutboundDispatcher(bus types.MessageBus, lookup func(string) (*ChannelAdapter, bool)) *OutboundDispatcher {
	return &OutboundDispatcher{
		bus:      bus,
		adapters: lookup,
	}
}

// Start begins the single outbound dispatch loop.
func (d *OutboundDispatcher) Start(ctx context.Context) {
	dctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	d.wg.Add(1)
	go d.run(dctx)
}

// Stop cancels the dispatch loop and waits for it to finish.
func (d *OutboundDispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
}

func (d *OutboundDispatcher) run(ctx context.Context) {
	defer d.wg.Done()
	for {
		msg, err := d.bus.ConsumeOutbound(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("outbound-dispatcher: consume error", "err", err)
			return
		}
		a, ok := d.adapters(msg.Channel)
		if !ok {
			slog.Warn("outbound-dispatcher: no adapter for channel", "channel", msg.Channel)
			continue
		}
		_ = a.Send(ctx, msg)
	}
}
