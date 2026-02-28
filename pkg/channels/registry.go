package channels

import (
	"context"
	"fmt"
	"sync"

	"abot/pkg/types"
)

// Registry manages multiple ChannelAdapters by name.
type Registry struct {
	adapters   map[string]*ChannelAdapter
	dispatcher *OutboundDispatcher
	mu         sync.RWMutex
}

// NewRegistry creates an empty channel registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]*ChannelAdapter),
	}
}

// Register adds a channel to the registry, creating an adapter that bridges it to the bus.
func (r *Registry) Register(name string, ch types.Channel, bus types.MessageBus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[name] = NewAdapter(ch, bus)
}

// Get returns the adapter for the given channel name.
func (r *Registry) Get(name string) (*ChannelAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

// Names returns all registered channel names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	return names
}

// StartAll starts every registered channel adapter and the outbound dispatcher.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var started []string
	for name, a := range r.adapters {
		if err := a.Start(ctx); err != nil {
			// rollback
			for _, s := range started {
				_ = r.adapters[s].Stop(ctx)
			}
			return fmt.Errorf("channels: start %s: %w", name, err)
		}
		started = append(started, name)
	}

	// Single outbound dispatcher for all channels — avoids competing-consumer bug.
	if len(r.adapters) > 0 {
		first := r.adapters[started[0]]
		r.dispatcher = NewOutboundDispatcher(first.bus, r.Get)
		r.dispatcher.Start(ctx)
	}

	return nil
}

// StopAll stops the outbound dispatcher and every registered channel adapter.
func (r *Registry) StopAll(ctx context.Context) error {
	if r.dispatcher != nil {
		r.dispatcher.Stop()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var firstErr error
	for name, a := range r.adapters {
		if err := a.Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("channels: stop %s: %w", name, err)
		}
	}
	return firstErr
}

// Unregister removes the named channel from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, name)
}

// GetStatus returns a snapshot of running status for all registered channels.
func (r *Registry) GetStatus() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]bool, len(r.adapters))
	for name, a := range r.adapters {
		status[name] = a.Channel().IsRunning()
	}
	return status
}

// SendToChannel sends a message directly to the named channel, bypassing the bus.
func (r *Registry) SendToChannel(ctx context.Context, channelName, chatID, content string) error {
	r.mu.RLock()
	a, ok := r.adapters[channelName]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channels: %s not found", channelName)
	}
	return a.Channel().Send(ctx, types.OutboundMessage{
		Channel: channelName,
		ChatID:  chatID,
		Content: content,
	})
}
