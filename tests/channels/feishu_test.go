package channels_test

import (
	"context"
	"testing"

	"abot/pkg/bus"
	"abot/pkg/channels/feishu"
	"abot/pkg/types"
)

// --- Constructor tests ---

func TestNewFeishuChannel_MissingAppID(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	_, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
		AppSecret:         "secret",
		VerificationToken: "tok",
	}, b)
	if err == nil {
		t.Fatal("expected error for missing app_id")
	}
}

func TestNewFeishuChannel_MissingVerificationToken(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	_, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
		AppID:     "id",
		AppSecret: "secret",
	}, b)
	if err == nil {
		t.Fatal("expected error for missing verification_token")
	}
}

func TestNewFeishuChannel_OK(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
		AppID:             "id",
		AppSecret:         "secret",
		VerificationToken: "tok",
	}, b)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name() != "feishu" {
		t.Errorf("name: %q", ch.Name())
	}
}

// --- Send tests ---

func TestFeishuChannel_Send_EmptyContent(t *testing.T) {
	b := bus.New(10)
	defer b.Close()

	ch, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
		AppID: "id", AppSecret: "secret", VerificationToken: "tok",
	}, b)
	if err != nil {
		t.Fatal(err)
	}
	err = ch.Send(context.Background(), types.OutboundMessage{ChatID: "c1", Content: ""})
	if err != nil {
		t.Errorf("empty content should not error: %v", err)
	}
}
