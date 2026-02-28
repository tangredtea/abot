package memoryconsolidation

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
)

// consolidate calls the LLM with save_memories tool to extract categorized memory entries.
func (c *consolidator) consolidate(
	ctx context.Context,
	conversation string,
	existingTenant, existingUser []VectorMemory,
	hasUser bool,
) ([]MemoryEntry, error) {

	prompt := BuildPrompt(conversation, existingTenant, existingUser, hasUser)

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: prompt}}},
		},
		Config: &genai.GenerateContentConfig{
			Temperature: genai.Ptr(float32(0.2)),
			Tools:       []*genai.Tool{SaveMemoryTool()},
		},
	}

	var resp *model.LLMResponse
	for r, err := range c.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return nil, fmt.Errorf("LLM error: %w", err)
		}
		resp = r
	}
	if resp == nil || resp.Content == nil {
		return nil, fmt.Errorf("empty LLM response")
	}

	return ParseToolCall(resp.Content)
}
