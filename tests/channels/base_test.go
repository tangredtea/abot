package channels_test

import (
	"context"
	"testing"

	"abot/pkg/channels"
)

func TestBaseChannel_Name(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, nil)
	if bc.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", bc.Name())
	}
}

func TestBaseChannel_IsAllowed_EmptyList(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, nil)
	if !bc.IsAllowed("anyone") {
		t.Fatal("empty allowlist should allow all")
	}
}

func TestBaseChannel_IsAllowed_Match(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, []string{"alice", "bob"})

	if !bc.IsAllowed("alice") {
		t.Fatal("alice should be allowed")
	}
	if !bc.IsAllowed("bob") {
		t.Fatal("bob should be allowed")
	}
	if bc.IsAllowed("eve") {
		t.Fatal("eve should not be allowed")
	}
}

func TestBaseChannel_IsAllowed_PipeSeparated(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, []string{"12345"})

	if !bc.IsAllowed("12345|alice") {
		t.Fatal("compound ID with matching part should be allowed")
	}
	if bc.IsAllowed("99999|eve") {
		t.Fatal("compound ID without matching part should be rejected")
	}
}

func TestBaseChannel_IsAllowed_CaseInsensitive(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, []string{"Alice"})

	if !bc.IsAllowed("alice") {
		t.Fatal("case-insensitive match should work")
	}
}

func TestBaseChannel_HandleMessage(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, nil)

	err := bc.HandleMessage(context.Background(), "t1", "u1", "sender", "chat1", "hello", nil, nil)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	msgs := bus.inboundMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 inbound message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Channel != "test" || m.TenantID != "t1" || m.Content != "hello" {
		t.Fatalf("unexpected message: %+v", m)
	}
}

func TestBaseChannel_HandleMessage_Blocked(t *testing.T) {
	bus := newMockBus()
	bc := channels.NewBaseChannel("test", bus, []string{"alice"})

	_ = bc.HandleMessage(context.Background(), "t1", "u1", "eve", "chat1", "hello", nil, nil)

	msgs := bus.inboundMessages()
	if len(msgs) != 0 {
		t.Fatal("blocked sender should produce no inbound messages")
	}
}
