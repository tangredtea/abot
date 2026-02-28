package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
)

// --- genai → OpenAI conversion ---

// ConvertRequest transforms an ADK LLMRequest into an OpenAI ChatCompletion request.
func ConvertRequest(req *model.LLMRequest, modelName string, stream bool) (*ChatRequest, error) {
	out := &ChatRequest{
		Model:  modelName,
		Stream: stream,
	}

	if stream {
		out.StreamOpts = &StreamOpts{IncludeUsage: true}
	}

	if req.Config != nil {
		if req.Config.Temperature != nil {
			t := float32(*req.Config.Temperature)
			out.Temperature = &t
		}
		if req.Config.TopP != nil {
			p := float32(*req.Config.TopP)
			out.TopP = &p
		}
		if req.Config.StopSequences != nil {
			out.Stop = req.Config.StopSequences
		}
		if req.Config.MaxOutputTokens > 0 {
			mt := int(req.Config.MaxOutputTokens)
			out.MaxTokens = &mt
		}
		if len(req.Config.Tools) > 0 {
			tools, err := convertTools(req.Config.Tools)
			if err != nil {
				return nil, err
			}
			out.Tools = tools
		}
	}

	msgs := ConvertContents(req.Config, req.Contents)
	out.Messages = msgs
	return out, nil
}

// convertTools converts genai.Tool declarations to OpenAI tool format.
func convertTools(tools []*genai.Tool) ([]ChatTool, error) {
	var out []ChatTool
	for _, t := range tools {
		for _, fd := range t.FunctionDeclarations {
			params, err := json.Marshal(ConvertSchema(fd.Parameters))
			if err != nil {
				return nil, fmt.Errorf("marshal tool %q params: %w", fd.Name, err)
			}
			out = append(out, ChatTool{
				Type: "function",
				Function: ChatFunction{
					Name:        fd.Name,
					Description: fd.Description,
					Parameters:  params,
				},
			})
		}
	}
	return out, nil
}

// ConvertSchema converts genai.Schema to a JSON-serializable map.
func ConvertSchema(s *genai.Schema) map[string]any {
	if s == nil {
		return map[string]any{"type": "object"}
	}
	m := map[string]any{
		"type": strings.ToLower(string(s.Type)),
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Items != nil {
		m["items"] = ConvertSchema(s.Items)
	}
	if len(s.Properties) > 0 {
		props := make(map[string]any, len(s.Properties))
		for k, v := range s.Properties {
			props[k] = ConvertSchema(v)
		}
		m["properties"] = props
	}
	return m
}

// ConvertContents converts genai.Content history to OpenAI chat messages.
func ConvertContents(cfg *genai.GenerateContentConfig, contents []*genai.Content) []ChatMessage {
	var msgs []ChatMessage

	// System instruction → system message.
	if cfg != nil && cfg.SystemInstruction != nil {
		var texts []string
		for _, p := range cfg.SystemInstruction.Parts {
			if p != nil && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		if len(texts) > 0 {
			joined := strings.Join(texts, "\n")
			raw, _ := json.Marshal(joined)
			msgs = append(msgs, ChatMessage{Role: "system", Content: raw})
		}
	}

	for _, c := range contents {
		if c == nil {
			continue
		}
		msgs = append(msgs, ConvertContent(c)...)
	}
	return msgs
}

// ConvertContent converts a single genai.Content to one or more OpenAI messages.
func ConvertContent(c *genai.Content) []ChatMessage {
	role := "user"
	if c.Role == "model" {
		role = "assistant"
	}

	// Collect text parts, function calls, and function responses separately.
	var textParts []string
	var toolCalls []ToolCallMsg
	var toolResults []ChatMessage

	for _, p := range c.Parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			textParts = append(textParts, p.Text)
		}
		if p.FunctionCall != nil {
			args, _ := json.Marshal(p.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCallMsg{
				ID:   p.FunctionCall.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      p.FunctionCall.Name,
					Arguments: string(args),
				},
			})
		}
		if p.FunctionResponse != nil {
			resp, _ := json.Marshal(p.FunctionResponse.Response)
			raw, _ := json.Marshal(string(resp))
			toolResults = append(toolResults, ChatMessage{
				Role:       "tool",
				ToolCallID: p.FunctionResponse.ID,
				Content:    raw,
			})
		}
	}

	var msgs []ChatMessage

	// Assistant message with text and/or tool calls.
	if role == "assistant" {
		msg := ChatMessage{Role: "assistant"}
		if len(textParts) > 0 {
			joined := strings.Join(textParts, "")
			raw, _ := json.Marshal(joined)
			msg.Content = raw
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		msgs = append(msgs, msg)
	} else if len(textParts) > 0 {
		// User message.
		joined := strings.Join(textParts, "\n")
		raw, _ := json.Marshal(joined)
		msgs = append(msgs, ChatMessage{Role: "user", Content: raw})
	}

	// Tool results as separate messages.
	msgs = append(msgs, toolResults...)
	return msgs
}

// --- OpenAI → genai conversion ---

// ConvertResponse transforms an OpenAI ChatCompletion response into an ADK LLMResponse.
func ConvertResponse(resp *ChatResponse) *model.LLMResponse {
	if len(resp.Choices) == 0 {
		return &model.LLMResponse{
			ErrorCode:    "empty_response",
			ErrorMessage: "openai: no choices in response",
			TurnComplete: true,
		}
	}

	choice := resp.Choices[0]
	content := convertChoiceToContent(&choice.Message)

	llmResp := &model.LLMResponse{
		Content:      content,
		TurnComplete: true,
		FinishReason: MapFinishReason(choice.FinishReason),
	}

	if resp.Usage != nil {
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.PromptTokens),
			CandidatesTokenCount: int32(resp.Usage.CompletionTokens),
			TotalTokenCount:      int32(resp.Usage.TotalTokens),
		}
	}

	return llmResp
}

// convertChoiceToContent converts an OpenAI message to genai.Content.
func convertChoiceToContent(msg *ChatMessage) *genai.Content {
	content := &genai.Content{
		Role: "model",
	}

	// Reasoning content from reasoning models (e.g. DeepSeek-R1, QwQ).
	// Prepend as a thinking block so downstream consumers can see the chain-of-thought.
	if msg.ReasoningContent != "" {
		content.Parts = append(content.Parts, &genai.Part{
			Text: "<thinking>\n" + msg.ReasoningContent + "\n</thinking>",
		})
	}

	// Text content.
	if len(msg.Content) > 0 {
		var text string
		_ = json.Unmarshal(msg.Content, &text)
		if text != "" {
			content.Parts = append(content.Parts, &genai.Part{Text: text})
		}
	}

	// Tool calls.
	for _, tc := range msg.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}

	return content
}

// MapFinishReason converts OpenAI finish_reason to genai.FinishReason.
func MapFinishReason(reason string) genai.FinishReason {
	switch reason {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	case "tool_calls":
		return genai.FinishReasonStop
	default:
		return genai.FinishReasonOther
	}
}

// newModelContent creates a genai.Content with role "model" from a single part.
func newModelContent(part *genai.Part) *genai.Content {
	return &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{part},
	}
}
