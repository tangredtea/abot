package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
)

// --- genai → Anthropic conversion ---

// ConvertRequest transforms an ADK LLMRequest into an Anthropic Messages API request.
// When promptCaching is true, cache_control is set on the last system block and last tool.
func ConvertRequest(req *model.LLMRequest, modelName string, stream bool, promptCaching bool) (*MessagesRequest, error) {
	out := &MessagesRequest{
		Model:     modelName,
		MaxTokens: 8192,
		Stream:    stream,
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
		if req.Config.TopK != nil {
			k := int32(*req.Config.TopK)
			out.TopK = &k
		}
		if req.Config.StopSequences != nil {
			out.StopSequences = req.Config.StopSequences
		}
		if req.Config.MaxOutputTokens > 0 {
			out.MaxTokens = int(req.Config.MaxOutputTokens)
		}
		if req.Config.SystemInstruction != nil {
			out.System = ConvertSystemInstruction(req.Config.SystemInstruction)
		}
		if len(req.Config.Tools) > 0 {
			out.Tools = convertTools(req.Config.Tools)
		}
	}

	msgs, err := ConvertContents(req.Contents)
	if err != nil {
		return nil, err
	}
	out.Messages = msgs

	// Apply prompt caching: mark the last system block and last tool as ephemeral.
	if promptCaching {
		if len(out.System) > 0 {
			out.System[len(out.System)-1].CacheControl = &CacheControl{Type: "ephemeral"}
		}
		if len(out.Tools) > 0 {
			out.Tools[len(out.Tools)-1].CacheControl = &CacheControl{Type: "ephemeral"}
		}
	}

	return out, nil
}

// ConvertSystemInstruction extracts text blocks from a genai.Content for the system field.
func ConvertSystemInstruction(c *genai.Content) []ContentBlock {
	if c == nil {
		return nil
	}
	var blocks []ContentBlock
	for _, p := range c.Parts {
		if p.Text != "" {
			blocks = append(blocks, ContentBlock{Type: "text", Text: p.Text})
		}
	}
	return blocks
}

// convertTools converts genai.Tool declarations to Anthropic tool format.
func convertTools(tools []*genai.Tool) []APITool {
	var out []APITool
	for _, t := range tools {
		for _, fd := range t.FunctionDeclarations {
			at := APITool{
				Name:        fd.Name,
				Description: fd.Description,
			}
			if fd.Parameters != nil {
				at.InputSchema = convertSchema(fd.Parameters)
			} else {
				at.InputSchema = &toolSchema{Type: "object"}
			}
			out = append(out, at)
		}
	}
	return out
}

// convertSchema converts a genai.Schema to Anthropic's tool schema format.
func convertSchema(s *genai.Schema) *toolSchema {
	if s == nil {
		return nil
	}
	ts := &toolSchema{
		Type:     strings.ToLower(string(s.Type)),
		Required: s.Required,
		Desc:     s.Description,
	}
	if len(s.Enum) > 0 {
		ts.Enum = s.Enum
	}
	if s.Items != nil {
		ts.Items = convertSchema(s.Items)
	}
	if len(s.Properties) > 0 {
		ts.Properties = make(map[string]*toolSchema, len(s.Properties))
		for k, v := range s.Properties {
			ts.Properties[k] = convertSchema(v)
		}
	}
	return ts
}

// ConvertContents converts genai.Content history to Anthropic messages.
// Merges consecutive same-role messages (Anthropic requires alternating roles).
func ConvertContents(contents []*genai.Content) ([]APIMessage, error) {
	var msgs []APIMessage

	for _, c := range contents {
		if c == nil {
			continue
		}
		role := MapRole(c.Role)
		blocks := convertParts(c.Parts, role)
		if len(blocks) == 0 {
			continue
		}

		// Merge with previous if same role.
		if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
			msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, blocks...)
		} else {
			msgs = append(msgs, APIMessage{Role: role, Content: blocks})
		}
	}

	return msgs, nil
}

// MapRole converts genai role to Anthropic role.
func MapRole(genaiRole string) string {
	if genaiRole == roleModel {
		return "assistant"
	}
	return "user"
}

// convertParts converts genai.Part slice to Anthropic content blocks.
func convertParts(parts []*genai.Part, role string) []ContentBlock {
	var blocks []ContentBlock
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" {
			blocks = append(blocks, ContentBlock{Type: "text", Text: p.Text})
		}
		if p.FunctionCall != nil {
			input, err := json.Marshal(p.FunctionCall.Args)
			if err != nil {
				slog.Warn("anthropic: marshal function call args", "name", p.FunctionCall.Name, "err", err)
				input = []byte("{}")
			}
			blocks = append(blocks, ContentBlock{
				Type:  "tool_use",
				ID:    p.FunctionCall.ID,
				Name:  p.FunctionCall.Name,
				Input: input,
			})
		}
		if p.FunctionResponse != nil {
			content, err := json.Marshal(p.FunctionResponse.Response)
			if err != nil {
				slog.Warn("anthropic: marshal function response", "id", p.FunctionResponse.ID, "err", err)
				content = []byte("{}")
			}
			blocks = append(blocks, ContentBlock{
				Type:      "tool_result",
				ToolUseID: p.FunctionResponse.ID,
				Content:   content,
			})
		}
		if p.InlineData != nil {
			blocks = append(blocks, ContentBlock{
				Type: "image",
				Source: &imageSource{
					Type:      "base64",
					MediaType: p.InlineData.MIMEType,
					Data:      base64.StdEncoding.EncodeToString(p.InlineData.Data),
				},
			})
		}
	}
	return blocks
}

// --- Anthropic → genai conversion ---

// ConvertResponse transforms an Anthropic response into an ADK LLMResponse.
func ConvertResponse(resp *MessagesResponse) *model.LLMResponse {
	content := &genai.Content{
		Role:  roleModel,
		Parts: make([]*genai.Part, 0, len(resp.Content)),
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content.Parts = append(content.Parts, &genai.Part{Text: block.Text})
		case "tool_use":
			var args map[string]any
			if len(block.Input) > 0 {
				_ = json.Unmarshal(block.Input, &args)
			}
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   block.ID,
					Name: block.Name,
					Args: args,
				},
			})
		}
	}

	llmResp := &model.LLMResponse{
		Content:      content,
		TurnComplete: true,
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.InputTokens),
			CandidatesTokenCount: int32(resp.Usage.OutputTokens),
			TotalTokenCount:      int32(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}

	llmResp.FinishReason = MapStopReason(resp.StopReason)
	return llmResp
}

// MapStopReason converts Anthropic stop_reason to genai.FinishReason.
func MapStopReason(reason string) genai.FinishReason {
	switch reason {
	case "end_turn":
		return genai.FinishReasonStop
	case "max_tokens":
		return genai.FinishReasonMaxTokens
	case "tool_use":
		return genai.FinishReasonStop
	default:
		return genai.FinishReasonOther
	}
}

// convertStreamDelta converts a streaming content_block_delta into a partial LLMResponse.
func convertStreamDelta(delta *streamDelta) *model.LLMResponse {
	content := &genai.Content{
		Role:  roleModel,
		Parts: make([]*genai.Part, 0, 1),
	}

	switch {
	case delta.Type == "text_delta":
		content.Parts = append(content.Parts, &genai.Part{Text: delta.Text})
	case delta.Type == "input_json_delta" && delta.PartialJSON != "":
		// Accumulate partial JSON for tool_use — emit as text partial.
		content.Parts = append(content.Parts, &genai.Part{Text: delta.PartialJSON})
	}

	return &model.LLMResponse{
		Content: content,
		Partial: true,
	}
}

// convertStreamToolUse builds a complete FunctionCall part from accumulated JSON.
func convertStreamToolUse(id, name, accumulatedJSON string) *genai.Part {
	var args map[string]any
	if accumulatedJSON != "" {
		_ = json.Unmarshal([]byte(accumulatedJSON), &args)
	}
	return &genai.Part{
		FunctionCall: &genai.FunctionCall{
			ID:   id,
			Name: name,
			Args: args,
		},
	}
}

// newModelContent creates a genai.Content with role "model" from a single part.
func newModelContent(part *genai.Part) *genai.Content {
	return &genai.Content{
		Role:  roleModel,
		Parts: []*genai.Part{part},
	}
}

// convertUsage converts Anthropic usage to genai usage metadata.
func convertUsage(u *APIUsage) *genai.GenerateContentResponseUsageMetadata {
	if u == nil {
		return nil
	}
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(u.InputTokens),
		CandidatesTokenCount: int32(u.OutputTokens),
		TotalTokenCount:      int32(u.InputTokens + u.OutputTokens),
	}
}
