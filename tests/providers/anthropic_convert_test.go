package providers_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/genai"

	"abot/pkg/providers/anthropic"
	"google.golang.org/adk/model"
)

func TestAnthropicMapRole(t *testing.T) {
	if anthropic.MapRole("model") != "assistant" {
		t.Error("model should map to assistant")
	}
	if anthropic.MapRole("user") != "user" {
		t.Error("user should map to user")
	}
	if anthropic.MapRole("anything") != "user" {
		t.Error("unknown role should default to user")
	}
}

func TestAnthropicMapStopReason(t *testing.T) {
	cases := []struct {
		in  string
		out genai.FinishReason
	}{
		{"end_turn", genai.FinishReasonStop},
		{"max_tokens", genai.FinishReasonMaxTokens},
		{"tool_use", genai.FinishReasonStop},
		{"unknown", genai.FinishReasonOther},
	}
	for _, c := range cases {
		if got := anthropic.MapStopReason(c.in); got != c.out {
			t.Errorf("MapStopReason(%q) = %v, want %v", c.in, got, c.out)
		}
	}
}

func TestAnthropicConvertSystemInstruction(t *testing.T) {
	c := &genai.Content{
		Parts: []*genai.Part{
			{Text: "You are helpful."},
			{Text: "Be concise."},
		},
	}
	blocks := anthropic.ConvertSystemInstruction(c)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "You are helpful." {
		t.Errorf("block 0: %+v", blocks[0])
	}
}

func TestAnthropicConvertSystemInstruction_Nil(t *testing.T) {
	if blocks := anthropic.ConvertSystemInstruction(nil); blocks != nil {
		t.Errorf("expected nil, got %v", blocks)
	}
}

func TestAnthropicConvertRequest_Basic(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(0.5)),
			MaxOutputTokens: 1024,
		},
	}
	out, err := anthropic.ConvertRequest(req, "claude-3", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "claude-3" {
		t.Errorf("model: %q", out.Model)
	}
	if out.MaxTokens != 1024 {
		t.Errorf("max_tokens: %d", out.MaxTokens)
	}
	if out.Temperature == nil || *out.Temperature != 0.5 {
		t.Error("temperature mismatch")
	}
	if len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Errorf("messages: %+v", out.Messages)
	}
}

func TestAnthropicConvertResponse_TextAndUsage(t *testing.T) {
	resp := &anthropic.MessagesResponse{
		Content: []anthropic.ContentBlock{
			{Type: "text", Text: "hi there"},
		},
		StopReason: "end_turn",
		Usage:      anthropic.APIUsage{InputTokens: 10, OutputTokens: 5},
	}
	out := anthropic.ConvertResponse(resp)
	if out.Content == nil || len(out.Content.Parts) != 1 {
		t.Fatal("expected 1 part")
	}
	if out.Content.Parts[0].Text != "hi there" {
		t.Errorf("text: %q", out.Content.Parts[0].Text)
	}
	if out.UsageMetadata.PromptTokenCount != 10 {
		t.Errorf("input tokens: %d", out.UsageMetadata.PromptTokenCount)
	}
	if out.FinishReason != genai.FinishReasonStop {
		t.Errorf("finish reason: %v", out.FinishReason)
	}
}

func TestAnthropicConvertResponse_ToolUse(t *testing.T) {
	resp := &anthropic.MessagesResponse{
		Content: []anthropic.ContentBlock{
			{Type: "tool_use", ID: "call_1", Name: "search", Input: json.RawMessage(`{"q":"test"}`)},
		},
		StopReason: "tool_use",
		Usage:      anthropic.APIUsage{InputTokens: 5, OutputTokens: 3},
	}
	out := anthropic.ConvertResponse(resp)
	if len(out.Content.Parts) != 1 {
		t.Fatal("expected 1 part")
	}
	fc := out.Content.Parts[0].FunctionCall
	if fc == nil || fc.Name != "search" || fc.ID != "call_1" {
		t.Errorf("function call: %+v", fc)
	}
}

func TestAnthropicConvertContents_MergesSameRole(t *testing.T) {
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "a"}}},
		{Role: "user", Parts: []*genai.Part{{Text: "b"}}},
	}
	msgs, err := anthropic.ConvertContents(contents)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 merged message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(msgs[0].Content))
	}
}

// --- Prompt caching tests ---

func TestAnthropicConvertRequest_PromptCaching(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: "You are helpful."},
					{Text: "Be concise."},
				},
			},
			Tools: []*genai.Tool{
				{FunctionDeclarations: []*genai.FunctionDeclaration{
					{Name: "search", Description: "Search"},
					{Name: "fetch", Description: "Fetch URL"},
				}},
			},
		},
	}

	// With caching enabled
	out, err := anthropic.ConvertRequest(req, "claude-3", false, true)
	if err != nil {
		t.Fatal(err)
	}

	// Last system block should have cache_control
	lastSys := out.System[len(out.System)-1]
	if lastSys.CacheControl == nil || lastSys.CacheControl.Type != "ephemeral" {
		t.Error("last system block should have cache_control=ephemeral")
	}
	// First system block should NOT have cache_control
	if out.System[0].CacheControl != nil {
		t.Error("first system block should not have cache_control")
	}

	// Last tool should have cache_control
	lastTool := out.Tools[len(out.Tools)-1]
	if lastTool.CacheControl == nil || lastTool.CacheControl.Type != "ephemeral" {
		t.Error("last tool should have cache_control=ephemeral")
	}
}

func TestAnthropicConvertRequest_NoCaching(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: "System prompt"}},
			},
		},
	}

	out, err := anthropic.ConvertRequest(req, "claude-3", false, false)
	if err != nil {
		t.Fatal(err)
	}

	// No cache_control when caching is disabled
	for _, s := range out.System {
		if s.CacheControl != nil {
			t.Error("cache_control should be nil when caching disabled")
		}
	}
}
