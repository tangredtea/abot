// Package guard provides tool result protection.
package guard

import (
	"fmt"

	"google.golang.org/genai"
)

// ToolResultGuard protects tool results from modification.
type ToolResultGuard struct{}

// NewToolResultGuard creates a tool result guard.
func NewToolResultGuard() *ToolResultGuard {
	return &ToolResultGuard{}
}

// Protect makes tool results read-only.
func (g *ToolResultGuard) Protect(content *genai.Content) error {
	if content == nil {
		return nil
	}

	// Validate tool results structure
	for _, part := range content.Parts {
		if part.FunctionResponse != nil {
			if err := g.validateToolResult(part); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateToolResult validates tool result structure.
func (g *ToolResultGuard) validateToolResult(part *genai.Part) error {
	if part.FunctionResponse == nil {
		return fmt.Errorf("missing function response")
	}

	if part.FunctionResponse.Name == "" {
		return fmt.Errorf("missing function name")
	}

	if part.FunctionResponse.Response == nil {
		return fmt.Errorf("missing response data")
	}

	return nil
}
