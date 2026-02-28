package memoryconsolidation

import (
	"google.golang.org/genai"
)

func SaveMemoryTool() *genai.Tool {
	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "save_memories",
				Description: "Save categorized memory entries extracted from conversation",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"entries": {
							Type:        genai.TypeArray,
							Description: "List of memory entries to save",
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"category": {
										Type:        genai.TypeString,
										Description: "Memory category, e.g. preference, fact, event, instruction, relationship, skill, goal",
									},
									"text": {
										Type:        genai.TypeString,
										Description: "The memory content, concise and self-contained",
									},
									"scope": {
										Type:        genai.TypeString,
										Description: "tenant (shared knowledge) or user (personal to the user)",
										Enum:        []string{"tenant", "user"},
									},
									"permanent": {
										Type:        genai.TypeBoolean,
										Description: "true for long-lived facts/preferences/instructions, false for temporal events. Omit to auto-determine by category.",
									},
								},
								Required: []string{"category", "text", "scope"},
							},
						},
					},
					Required: []string{"entries"},
				},
			},
		},
	}
}
