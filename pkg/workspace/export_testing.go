//go:build testing

package workspace

// NewPromptLayer creates a PromptLayer for external testing.
func NewPromptLayer(content string, priority int) PromptLayer {
	return PromptLayer{content: content, priority: priority}
}

// Priority returns the priority of a PromptLayer for external testing.
func (l PromptLayer) Priority() int {
	return l.priority
}
