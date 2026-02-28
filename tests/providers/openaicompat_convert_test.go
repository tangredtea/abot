package providers_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/genai"

	"abot/pkg/providers/openaicompat"
	"google.golang.org/adk/model"
)

func TestOpenAIMapFinishReason(t *testing.T) {
	cases := []struct {
		in  string
		out genai.FinishReason
	}{
		{"stop", genai.FinishReasonStop},
		{"length", genai.FinishReasonMaxTokens},
		{"tool_calls", genai.FinishReasonStop},
		{"unknown", genai.FinishReasonOther},
	}
	for _, c := range cases {
		if got := openaicompat.MapFinishReason(c.in); got != c.out {
			t.Errorf("MapFinishReason(%q) = %v, want %v", c.in, got, c.out)
		}
	}
}

func TestOpenAIConvertRequest_Basic(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr(float32(0.7)),
			MaxOutputTokens: 2048,
		},
	}
	out, err := openaicompat.ConvertRequest(req, "gpt-4", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "gpt-4" {
		t.Errorf("model: %q", out.Model)
	}
	if out.MaxTokens == nil || *out.MaxTokens != 2048 {
		t.Error("max_tokens mismatch")
	}
	if out.Temperature == nil || *out.Temperature != 0.7 {
		t.Error("temperature mismatch")
	}
}

func TestOpenAIConvertResponse_Text(t *testing.T) {
	text, _ := json.Marshal("hello world")
	resp := &openaicompat.ChatResponse{
		Choices: []openaicompat.ChatChoice{
			{
				Message:      openaicompat.ChatMessage{Role: "assistant", Content: text},
				FinishReason: "stop",
			},
		},
		Usage: &openaicompat.ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	out := openaicompat.ConvertResponse(resp)
	if len(out.Content.Parts) != 1 || out.Content.Parts[0].Text != "hello world" {
		t.Errorf("text mismatch: %+v", out.Content.Parts)
	}
	if out.UsageMetadata.PromptTokenCount != 10 {
		t.Errorf("prompt tokens: %d", out.UsageMetadata.PromptTokenCount)
	}
	if out.FinishReason != genai.FinishReasonStop {
		t.Errorf("finish reason: %v", out.FinishReason)
	}
}

func TestOpenAIConvertResponse_EmptyChoices(t *testing.T) {
	resp := &openaicompat.ChatResponse{Choices: nil}
	out := openaicompat.ConvertResponse(resp)
	if out.ErrorCode != "empty_response" {
		t.Errorf("expected empty_response error, got %q", out.ErrorCode)
	}
}

func TestOpenAIConvertResponse_ToolCalls(t *testing.T) {
	resp := &openaicompat.ChatResponse{
		Choices: []openaicompat.ChatChoice{
			{
				Message: openaicompat.ChatMessage{
					Role: "assistant",
					ToolCalls: []openaicompat.ToolCallMsg{
						{ID: "tc_1", Type: "function", Function: openaicompat.FunctionCall{Name: "search", Arguments: `{"q":"test"}`}},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	out := openaicompat.ConvertResponse(resp)
	if len(out.Content.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(out.Content.Parts))
	}
	fc := out.Content.Parts[0].FunctionCall
	if fc == nil || fc.Name != "search" || fc.ID != "tc_1" {
		t.Errorf("function call: %+v", fc)
	}
}

func TestOpenAIConvertResponse_ReasoningContent(t *testing.T) {
	text, _ := json.Marshal("final answer")
	resp := &openaicompat.ChatResponse{
		Choices: []openaicompat.ChatChoice{
			{
				Message: openaicompat.ChatMessage{
					Role:             "assistant",
					Content:          text,
					ReasoningContent: "let me think step by step",
				},
				FinishReason: "stop",
			},
		},
	}
	out := openaicompat.ConvertResponse(resp)
	if len(out.Content.Parts) != 2 {
		t.Fatalf("expected 2 parts (thinking + text), got %d", len(out.Content.Parts))
	}
	// First part: reasoning wrapped in <thinking> tags
	thinking := out.Content.Parts[0].Text
	if thinking != "<thinking>\nlet me think step by step\n</thinking>" {
		t.Errorf("reasoning part: %q", thinking)
	}
	// Second part: actual text
	if out.Content.Parts[1].Text != "final answer" {
		t.Errorf("text part: %q", out.Content.Parts[1].Text)
	}
}

func TestOpenAIConvertContents_SystemInstruction(t *testing.T) {
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: "You are helpful."},
				{Text: "Be concise."},
			},
		},
	}
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "hi"}}},
	}
	msgs := openaicompat.ConvertContents(cfg, contents)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message role: %q", msgs[0].Role)
	}
	var sysText string
	json.Unmarshal(msgs[0].Content, &sysText)
	if sysText != "You are helpful.\nBe concise." {
		t.Errorf("system text: %q", sysText)
	}
}

func TestOpenAIConvertContent_ToolResults(t *testing.T) {
	c := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{FunctionResponse: &genai.FunctionResponse{
				ID:       "tc_1",
				Name:     "search",
				Response: map[string]any{"result": "found"},
			}},
		},
	}
	msgs := openaicompat.ConvertContent(c)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 tool message, got %d", len(msgs))
	}
	if msgs[0].Role != "tool" {
		t.Errorf("role: %q", msgs[0].Role)
	}
	if msgs[0].ToolCallID != "tc_1" {
		t.Errorf("tool_call_id: %q", msgs[0].ToolCallID)
	}
}

func TestOpenAIConvertSchema_Nil(t *testing.T) {
	m := openaicompat.ConvertSchema(nil)
	if m["type"] != "object" {
		t.Errorf("nil schema should default to object, got %v", m["type"])
	}
}

func TestOpenAIConvertSchema_WithProperties(t *testing.T) {
	s := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"q"},
		Properties: map[string]*genai.Schema{
			"q": {Type: genai.TypeString, Description: "query"},
		},
	}
	m := openaicompat.ConvertSchema(s)
	if m["type"] != "object" {
		t.Errorf("type: %v", m["type"])
	}
	req, ok := m["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "q" {
		t.Errorf("required: %v", m["required"])
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing")
	}
	qProp, ok := props["q"].(map[string]any)
	if !ok || qProp["type"] != "string" {
		t.Errorf("q property: %v", props["q"])
	}
}
