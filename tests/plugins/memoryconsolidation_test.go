package plugins_test

import (
	"strings"
	"testing"

	"google.golang.org/genai"

	mc "abot/pkg/plugins/memoryconsolidation"
)

// --- prompt tests ---

func TestBuildPrompt_Basic(t *testing.T) {
	p := mc.BuildPrompt("hello world", nil, nil, false)
	if !strings.Contains(p, "hello world") {
		t.Error("prompt should contain conversation")
	}
	if !strings.Contains(p, "scope \"tenant\"") {
		t.Error("should instruct all entries as tenant when hasUser=false")
	}
}

func TestBuildPrompt_WithExistingMemory(t *testing.T) {
	tenant := []mc.VectorMemory{
		{Category: "fact", Text: "tenant facts"},
	}
	user := []mc.VectorMemory{
		{Category: "preference", Text: "user prefs"},
	}
	p := mc.BuildPrompt("conv", tenant, user, true)
	if !strings.Contains(p, "Existing Tenant Memories") {
		t.Error("should include tenant memory section")
	}
	if !strings.Contains(p, "tenant facts") {
		t.Error("should include tenant memory content")
	}
	if !strings.Contains(p, "Existing User Memories") {
		t.Error("should include user memory section")
	}
	if !strings.Contains(p, "user prefs") {
		t.Error("should include user memory content")
	}
}

func TestBuildPrompt_NoUserSection(t *testing.T) {
	tenant := []mc.VectorMemory{
		{Category: "fact", Text: "tenant"},
	}
	p := mc.BuildPrompt("conv", tenant, nil, false)
	if strings.Contains(p, "Existing User Memories") {
		t.Error("should not include user memory section when hasUser=false")
	}
}

func TestBuildPrompt_EmptyExisting(t *testing.T) {
	p := mc.BuildPrompt("conv", nil, nil, true)
	if strings.Contains(p, "Existing Tenant Memories") {
		t.Error("should not include empty tenant memory section")
	}
}

// --- parse tests ---

func TestParseToolCall_WithEntries(t *testing.T) {
	content := &genai.Content{
		Parts: []*genai.Part{
			{
				FunctionCall: &genai.FunctionCall{
					Name: "save_memories",
					Args: map[string]any{
						"entries": []any{
							map[string]any{
								"category": "fact",
								"text":     "The project uses Go 1.24",
								"scope":    "tenant",
							},
							map[string]any{
								"category": "preference",
								"text":     "User prefers dark mode",
								"scope":    "user",
							},
						},
					},
				},
			},
		},
	}

	entries, err := mc.ParseToolCall(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Category != "fact" {
		t.Errorf("entry[0] category: %q", entries[0].Category)
	}
	if entries[0].Scope != "tenant" {
		t.Errorf("entry[0] scope: %q", entries[0].Scope)
	}
	if entries[1].Category != "preference" {
		t.Errorf("entry[1] category: %q", entries[1].Category)
	}
	if entries[1].Scope != "user" {
		t.Errorf("entry[1] scope: %q", entries[1].Scope)
	}
}

func TestParseToolCall_NoToolCall(t *testing.T) {
	content := &genai.Content{
		Parts: []*genai.Part{
			{Text: "just text, no tool call"},
		},
	}
	_, err := mc.ParseToolCall(content)
	if err == nil {
		t.Error("expected error when no save_memories call")
	}
}

func TestParseToolCall_WrongToolName(t *testing.T) {
	content := &genai.Content{
		Parts: []*genai.Part{
			{
				FunctionCall: &genai.FunctionCall{
					Name: "other_tool",
					Args: map[string]any{"entries": []any{}},
				},
			},
		},
	}
	_, err := mc.ParseToolCall(content)
	if err == nil {
		t.Error("expected error for wrong tool name")
	}
}

func TestParseToolCall_EmptyTextSkipped(t *testing.T) {
	content := &genai.Content{
		Parts: []*genai.Part{
			{
				FunctionCall: &genai.FunctionCall{
					Name: "save_memories",
					Args: map[string]any{
						"entries": []any{
							map[string]any{
								"category": "fact",
								"text":     "",
								"scope":    "tenant",
							},
						},
					},
				},
			},
		},
	}
	entries, err := mc.ParseToolCall(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (empty text skipped), got %d", len(entries))
	}
}

// --- tool definition tests ---

func TestSaveMemoryTool_Structure(t *testing.T) {
	tool := mc.SaveMemoryTool()
	if len(tool.FunctionDeclarations) != 1 {
		t.Fatal("expected 1 function declaration")
	}
	fd := tool.FunctionDeclarations[0]
	if fd.Name != "save_memories" {
		t.Errorf("expected name save_memories, got %q", fd.Name)
	}
	entries, ok := fd.Parameters.Properties["entries"]
	if !ok {
		t.Fatal("missing entries property")
	}
	if entries.Items == nil {
		t.Fatal("entries should have Items schema")
	}
	item := entries.Items
	for _, required := range []string{"category", "text", "scope"} {
		if _, ok := item.Properties[required]; !ok {
			t.Errorf("missing required property: %s", required)
		}
	}
}
