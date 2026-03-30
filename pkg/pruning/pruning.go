// Package pruning provides three-layer progressive context pruning.
//
// Inspired by OpenClaw's context-pruning strategy:
// Layer 1: Soft Trim - Keep head + tail of tool results
// Layer 2: Hard Clear - Replace with placeholder
// Layer 3: Message Drop - Drop old messages, protect recent ones
package pruning

import (
	"strings"

	"google.golang.org/genai"
)

// Strategy defines pruning thresholds.
type Strategy struct {
	SoftTrimRatio  float64 // Trigger soft trim when ratio exceeds (default 0.3)
	HardClearRatio float64 // Trigger hard clear when ratio exceeds (default 0.5)
	KeepRecentN    int     // Protect N recent assistant messages (default 5)
	HeadChars      int     // Characters to keep at head (default 500)
	TailChars      int     // Characters to keep at tail (default 500)
}

// DefaultStrategy returns recommended pruning settings.
func DefaultStrategy() Strategy {
	return Strategy{
		SoftTrimRatio:  0.3,
		HardClearRatio: 0.5,
		KeepRecentN:    5,
		HeadChars:      500,
		TailChars:      500,
	}
}

// Prune applies three-layer progressive pruning to messages.
func (s Strategy) Prune(messages []*genai.Content, targetTokens int) []*genai.Content {
	currentTokens := estimateTokens(messages)
	if currentTokens <= targetTokens {
		return messages
	}

	// Layer 1: Soft trim tool results
	messages = s.softTrimToolResults(messages)
	currentTokens = estimateTokens(messages)
	if currentTokens <= targetTokens {
		return messages
	}

	// Layer 2: Hard clear tool results
	messages = s.hardClearToolResults(messages)
	currentTokens = estimateTokens(messages)
	if currentTokens <= targetTokens {
		return messages
	}

	// Layer 3: Drop old messages
	return s.dropOldMessages(messages, targetTokens)
}

// softTrimToolResults keeps head + tail of tool result content.
func (s Strategy) softTrimToolResults(messages []*genai.Content) []*genai.Content {
	result := make([]*genai.Content, len(messages))
	for i, msg := range messages {
		result[i] = s.softTrimMessage(msg)
	}
	return result
}

func (s Strategy) softTrimMessage(msg *genai.Content) *genai.Content {
	if msg.Role != "user" {
		return msg
	}

	newParts := make([]*genai.Part, len(msg.Parts))
	for i, part := range msg.Parts {
		if part.FunctionResponse != nil && part.FunctionResponse.Response != nil {
			// Trim function response content
			newParts[i] = s.softTrimPart(part)
		} else {
			newParts[i] = part
		}
	}

	return &genai.Content{
		Role:  msg.Role,
		Parts: newParts,
	}
}

func (s Strategy) softTrimPart(part *genai.Part) *genai.Part {
	// Extract text content from response
	content := extractTextContent(part.FunctionResponse.Response)
	if len(content) <= s.HeadChars+s.TailChars {
		return part
	}

	// Keep head + tail
	head := content[:s.HeadChars]
	tail := content[len(content)-s.TailChars:]
	trimmed := head + "\n... [truncated] ...\n" + tail

	// Create new part with trimmed content
	return &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			Name:     part.FunctionResponse.Name,
			Response: map[string]any{"content": trimmed},
		},
	}
}

// hardClearToolResults replaces tool result content with placeholder.
func (s Strategy) hardClearToolResults(messages []*genai.Content) []*genai.Content {
	result := make([]*genai.Content, len(messages))
	for i, msg := range messages {
		result[i] = s.hardClearMessage(msg)
	}
	return result
}

func (s Strategy) hardClearMessage(msg *genai.Content) *genai.Content {
	if msg.Role != "user" {
		return msg
	}

	newParts := make([]*genai.Part, len(msg.Parts))
	for i, part := range msg.Parts {
		if part.FunctionResponse != nil {
			newParts[i] = &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     part.FunctionResponse.Name,
					Response: map[string]any{"content": "[Old tool result content cleared]"},
				},
			}
		} else {
			newParts[i] = part
		}
	}

	return &genai.Content{
		Role:  msg.Role,
		Parts: newParts,
	}
}

// dropOldMessages drops old messages while protecting recent assistant messages.
func (s Strategy) dropOldMessages(messages []*genai.Content, targetTokens int) []*genai.Content {
	// Find recent assistant messages to protect
	protectedIndices := s.findRecentAssistantIndices(messages)

	// Drop from oldest, skip protected
	result := []*genai.Content{}
	currentTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		tokens := estimateTokens([]*genai.Content{msg})

		if currentTokens+tokens <= targetTokens || protectedIndices[i] {
			result = append([]*genai.Content{msg}, result...)
			currentTokens += tokens
		}
	}

	return result
}

func (s Strategy) findRecentAssistantIndices(messages []*genai.Content) map[int]bool {
	indices := make(map[int]bool)
	count := 0

	for i := len(messages) - 1; i >= 0 && count < s.KeepRecentN; i-- {
		if messages[i].Role == "model" {
			indices[i] = true
			count++
		}
	}

	return indices
}

// estimateTokens estimates token count (4 chars ≈ 1 token).
func estimateTokens(messages []*genai.Content) int {
	chars := 0
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if part.Text != "" {
				chars += len(part.Text)
			}
			if part.FunctionResponse != nil {
				chars += len(extractTextContent(part.FunctionResponse.Response))
			}
		}
	}
	return chars / 4
}

func extractTextContent(response map[string]any) string {
	if response == nil {
		return ""
	}
	if content, ok := response["content"].(string); ok {
		return content
	}
	// Fallback: stringify the whole response
	var sb strings.Builder
	for k, v := range response {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(toString(v))
		sb.WriteString("\n")
	}
	return sb.String()
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
