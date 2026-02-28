package channels_test

import (
	"context"
	"testing"

	"abot/pkg/bus"
	"abot/pkg/channels/discord"
	"abot/pkg/types"
)

// --- Constructor tests ---

func TestNewDiscordChannel_MissingToken(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	_, err := discord.NewDiscordChannel(discord.DiscordConfig{}, b)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNewDiscordChannel_OK(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := discord.NewDiscordChannel(discord.DiscordConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name() != "discord" {
		t.Errorf("name: %q", ch.Name())
	}
}

func TestNewDiscordChannel_CustomTenant(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := discord.NewDiscordChannel(discord.DiscordConfig{
		Token:    "fake",
		TenantID: "tenant-42",
	}, b)
	if err != nil {
		t.Fatal(err)
	}
	_ = ch
}

// --- Send tests ---

func TestDiscordChannel_Send_EmptyContent(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := discord.NewDiscordChannel(discord.DiscordConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}
	err = ch.Send(context.Background(), types.OutboundMessage{ChatID: "123", Content: ""})
	if err != nil {
		t.Errorf("empty content should not error: %v", err)
	}
}

// --- Start/Stop lifecycle ---

func TestDiscordChannel_StartStop(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := discord.NewDiscordChannel(discord.DiscordConfig{Token: "fake"}, b)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Double start is no-op
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	cancel()
	if err := ch.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Double stop is no-op
	if err := ch.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
