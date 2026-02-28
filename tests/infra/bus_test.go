package infra_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"abot/pkg/bus"
	"abot/pkg/types"
)

// --- Bus pub/sub tests ---

func TestBus_InboundRoundTrip(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ctx := context.Background()
	msg := types.InboundMessage{TenantID: "t1", UserID: "u1", Content: "hello"}

	if err := b.PublishInbound(ctx, msg); err != nil {
		t.Fatal(err)
	}
	got, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello" || got.TenantID != "t1" {
		t.Errorf("got %+v", got)
	}
}

func TestBus_OutboundRoundTrip(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ctx := context.Background()
	msg := types.OutboundMessage{Channel: "cli", ChatID: "c1", Content: "reply"}

	if err := b.PublishOutbound(ctx, msg); err != nil {
		t.Fatal(err)
	}
	got, err := b.ConsumeOutbound(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "reply" || got.Channel != "cli" {
		t.Errorf("got %+v", got)
	}
}

func TestBus_ConcurrentProduceConsume(t *testing.T) {
	b := bus.New(100)
	defer b.Close()

	ctx := context.Background()
	const n = 50
	var wg sync.WaitGroup

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			b.PublishInbound(ctx, types.InboundMessage{Content: "msg"})
		}
	}()

	// Consumer
	count := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			if _, err := b.ConsumeInbound(ctx); err == nil {
				count++
			}
		}
	}()

	wg.Wait()
	if count != n {
		t.Errorf("consumed %d, want %d", count, n)
	}
}

func TestBus_CloseUnblocksConsumers(t *testing.T) {
	b := bus.New(10)

	done := make(chan struct{})
	go func() {
		_, err := b.ConsumeInbound(context.Background())
		if err != bus.ErrBusClosed {
			t.Errorf("expected ErrBusClosed, got %v", err)
		}
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	b.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer not unblocked after Close")
	}
}

func TestBus_PublishAfterClose(t *testing.T) {
	b := bus.New(10)
	b.Close()

	err := b.PublishInbound(context.Background(), types.InboundMessage{})
	if err != bus.ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got %v", err)
	}
}

func TestBus_DoubleCloseIsNoop(t *testing.T) {
	b := bus.New(10)
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBus_QueueSize(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ctx := context.Background()
	b.PublishInbound(ctx, types.InboundMessage{Content: "a"})
	b.PublishInbound(ctx, types.InboundMessage{Content: "b"})

	if b.InboundSize() != 2 {
		t.Errorf("inbound size: %d", b.InboundSize())
	}
	if b.OutboundSize() != 0 {
		t.Errorf("outbound size: %d", b.OutboundSize())
	}
}

func TestBus_ContextCancel(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := b.ConsumeInbound(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestBus_PublishContextCancel(t *testing.T) {
	b := bus.New(1) // buffer of 1

	ctx := context.Background()
	// fill the buffer
	_ = b.PublishInbound(ctx, types.InboundMessage{Content: "fill"})

	// next publish should block; use cancelled context
	ctx2, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.PublishInbound(ctx2, types.InboundMessage{Content: "blocked"})
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	b.Close()
}

func TestBus_DrainAfterClose(t *testing.T) {
	b := bus.New(10)
	ctx := context.Background()

	// publish before close
	_ = b.PublishInbound(ctx, types.InboundMessage{Content: "pre-close"})
	b.Close()

	// should still be able to drain the buffered message
	got, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("drain after close: %v", err)
	}
	if got.Content != "pre-close" {
		t.Fatalf("unexpected: %+v", got)
	}
}
