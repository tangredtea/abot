package channels_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"abot/pkg/channels/cli"
	"abot/pkg/types"
)

func TestCLI_SendWritesToWriter(t *testing.T) {
	bus := newMockBus()
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(""), &out, "t1", "u1")

	err := ch.Send(context.Background(), types.OutboundMessage{Content: "hello"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestCLI_ReadLoopPublishesMessages(t *testing.T) {
	bus := newMockBus()
	input := "hello world\ngoodbye\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for read loop to finish (EOF on reader)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 inbound messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello world" {
		t.Fatalf("expected 'hello world', got %q", msgs[0].Content)
	}
	if msgs[1].Content != "goodbye" {
		t.Fatalf("expected 'goodbye', got %q", msgs[1].Content)
	}
	if msgs[0].Channel != "cli" || msgs[0].TenantID != "t1" {
		t.Fatalf("unexpected message fields: %+v", msgs[0])
	}
}

func TestCLI_SlashHelp(t *testing.T) {
	bus := newMockBus()
	input := "/help\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 0 {
		t.Fatalf("slash command should not produce inbound messages, got %d", len(msgs))
	}
	if !strings.Contains(out.String(), "/help") {
		t.Fatal("expected help text in output")
	}
}

func TestCLI_SlashNew(t *testing.T) {
	bus := newMockBus()
	input := "/new\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 0 {
		t.Fatal("/new should not produce inbound messages")
	}
	if !strings.Contains(out.String(), "session reset") {
		t.Fatal("expected 'session reset' in output")
	}
}

func TestCLI_ExitCommand(t *testing.T) {
	bus := newMockBus()
	input := "exit\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 0 {
		t.Fatal("exit should not produce inbound messages")
	}
	if !strings.Contains(out.String(), "bye") {
		t.Fatal("expected 'bye' in output")
	}
}

func TestCLI_SlashStatus_Default(t *testing.T) {
	bus := newMockBus()
	input := "/status\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 0 {
		t.Fatal("/status should not produce inbound messages")
	}
	if !strings.Contains(out.String(), "status: running") {
		t.Fatalf("expected default status text, got %q", out.String())
	}
}

func TestCLI_SlashStatus_WithProvider(t *testing.T) {
	bus := newMockBus()
	input := "/status\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1",
		cli.WithStatusProvider(func() string { return "bus: inbound=3 outbound=1" }),
	)

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	if !strings.Contains(out.String(), "bus: inbound=3 outbound=1") {
		t.Fatalf("expected custom status, got %q", out.String())
	}
}

func TestCLI_NoMarkdown_StripsBoldAndHeadings(t *testing.T) {
	bus := newMockBus()
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(""), &out, "t1", "u1",
		cli.WithNoMarkdown(true),
	)

	err := ch.Send(context.Background(), types.OutboundMessage{Content: "## Hello **world**"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if strings.Contains(got, "**") || strings.Contains(got, "##") {
		t.Fatalf("markdown not stripped: %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Fatalf("content lost after stripping: %q", got)
	}
}

func TestCLI_NoMarkdown_PreservesCodeBlockContent(t *testing.T) {
	bus := newMockBus()
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(""), &out, "t1", "u1",
		cli.WithNoMarkdown(true),
	)

	input := "```go\nfmt.Println(\"hello\")\n```"
	err := ch.Send(context.Background(), types.OutboundMessage{Content: input})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "fmt.Println") {
		t.Fatalf("code block content lost: %q", got)
	}
	if strings.Contains(got, "```") {
		t.Fatalf("code fences should be stripped: %q", got)
	}
}

func TestCLI_EmptyLinesSkipped(t *testing.T) {
	bus := newMockBus()
	input := "\n\nhello\n\n"
	var out bytes.Buffer
	ch := cli.NewCLI(bus, strings.NewReader(input), &out, "t1", "u1")

	ctx := context.Background()
	ch.Start(ctx)
	ch.Stop(ctx)

	msgs := bus.inboundMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (empty lines skipped), got %d", len(msgs))
	}
}
