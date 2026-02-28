package memoryconsolidation

import (
	"fmt"
	"strings"

	"google.golang.org/adk/session"
)

// extractConversation builds a text transcript from session events.
func (c *consolidator) extractConversation(sess session.Session) string {
	var b strings.Builder
	for event := range sess.Events().All() {
		if event.Content == nil {
			continue
		}
		author := event.Author
		if author == "" {
			author = "system"
		}
		for _, part := range event.Content.Parts {
			if part.Text != "" {
				fmt.Fprintf(&b, "[%s] %s\n", author, part.Text)
			}
			if part.FunctionCall != nil {
				fmt.Fprintf(&b, "[%s] called tool: %s\n", author, part.FunctionCall.Name)
			}
		}
	}
	return b.String()
}
