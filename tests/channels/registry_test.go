package channels_test

import (
	"context"
	"testing"

	"abot/pkg/channels"
	"abot/pkg/types"
)

// stubChannel is a minimal Channel implementation for registry tests.
type stubChannel struct {
	name    string
	started bool
	stopped bool
	sent    []types.OutboundMessage
}

func (s *stubChannel) Name() string { return s.name }
func (s *stubChannel) Start(_ context.Context) error {
	s.started = true
	return nil
}
func (s *stubChannel) Stop(_ context.Context) error {
	s.stopped = true
	return nil
}
func (s *stubChannel) Send(_ context.Context, msg types.OutboundMessage) error {
	s.sent = append(s.sent, msg)
	return nil
}
func (s *stubChannel) IsAllowed(_ string) bool { return true }
func (s *stubChannel) IsRunning() bool         { return s.started && !s.stopped }

func TestRegistry_RegisterAndGet(t *testing.T) {
	bus := newMockBus()
	reg := channels.NewRegistry()
	ch := &stubChannel{name: "test"}

	reg.Register("test", ch, bus)

	a, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected to find registered channel")
	}
	if a.Channel().Name() != "test" {
		t.Fatalf("expected channel name 'test', got %q", a.Channel().Name())
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	reg := channels.NewRegistry()
	_, ok := reg.Get("nope")
	if ok {
		t.Fatal("expected missing channel to return false")
	}
}

func TestRegistry_Names(t *testing.T) {
	bus := newMockBus()
	reg := channels.NewRegistry()
	reg.Register("a", &stubChannel{name: "a"}, bus)
	reg.Register("b", &stubChannel{name: "b"}, bus)

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}

func TestRegistry_StartAllStopAll(t *testing.T) {
	bus := newMockBus()
	reg := channels.NewRegistry()
	ch1 := &stubChannel{name: "ch1"}
	ch2 := &stubChannel{name: "ch2"}
	reg.Register("ch1", ch1, bus)
	reg.Register("ch2", ch2, bus)

	ctx := context.Background()
	if err := reg.StartAll(ctx); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	if !ch1.started || !ch2.started {
		t.Fatal("expected both channels to be started")
	}

	if err := reg.StopAll(ctx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	if !ch1.stopped || !ch2.stopped {
		t.Fatal("expected both channels to be stopped")
	}
}
