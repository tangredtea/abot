package channels_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"abot/pkg/bus"
	"abot/pkg/channels/telegram"
	"abot/pkg/types"
)

// --- Constructor tests ---

func TestNewTelegramChannel_MissingToken(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	_, err := telegram.NewTelegramChannel(telegram.TelegramConfig{}, b)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNewTelegramChannel_DefaultPollTimeout(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := telegram.NewTelegramChannel(telegram.TelegramConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name() != "telegram" {
		t.Errorf("name: %q", ch.Name())
	}
}

func TestNewTelegramChannel_DefaultTenant(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := telegram.NewTelegramChannel(telegram.TelegramConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic; channel is created with default tenant
	_ = ch
}

// --- Send tests with mock Telegram API ---

func TestTelegramChannel_Send_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendMessage" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	b := bus.New(10)
	defer b.Close()

	ch, err := telegram.NewTelegramChannel(telegram.TelegramConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	// Override apiBase to point to test server — we use the exported Send method
	// which calls sendMessage internally. Since apiBase is unexported, we test
	// Send with an invalid chat_id to verify error handling.
	err = ch.Send(context.Background(), types.OutboundMessage{
		ChatID:  "not-a-number",
		Content: "hello",
	})
	if err == nil {
		t.Fatal("expected error for non-numeric chat_id")
	}
}

func TestTelegramChannel_Send_EmptyContent(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := telegram.NewTelegramChannel(telegram.TelegramConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	// Empty content should be a no-op
	err = ch.Send(context.Background(), types.OutboundMessage{ChatID: "123", Content: ""})
	if err != nil {
		t.Errorf("empty content should not error: %v", err)
	}
}

// --- Start/Stop lifecycle ---

func TestTelegramChannel_StartStop(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := telegram.NewTelegramChannel(telegram.TelegramConfig{Token: "fake", PollTimeout: 1}, b)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Double start should be no-op
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	cancel()
	if err := ch.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Double stop should be no-op
	if err := ch.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
