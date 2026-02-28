package memoryconsolidation

import (
	"fmt"

	"google.golang.org/genai"
)

// parseToolCall extracts categorized memory entries from the LLM's function call response.
func ParseToolCall(content *genai.Content) ([]MemoryEntry, error) {
	for _, part := range content.Parts {
		if part.FunctionCall == nil || part.FunctionCall.Name != "save_memories" {
			continue
		}
		return parseEntries(part.FunctionCall.Args)
	}
	return nil, fmt.Errorf("no save_memories tool call in LLM response")
}

func parseEntries(args map[string]any) ([]MemoryEntry, error) {
	raw, ok := args["entries"]
	if !ok {
		return nil, fmt.Errorf("missing entries field")
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("entries is not an array")
	}

	var entries []MemoryEntry
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		e := MemoryEntry{
			Category: strVal(m, "category"),
			Text:     strVal(m, "text"),
			Scope:    strVal(m, "scope"),
		}
		if p, ok := m["permanent"].(bool); ok {
			e.Permanent = &p
		}
		if e.Text == "" {
			continue
		}
		if e.Scope == "" {
			e.Scope = "tenant"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
