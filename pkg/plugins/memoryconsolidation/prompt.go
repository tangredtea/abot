package memoryconsolidation

import (
	"fmt"
	"strings"
)

// VectorMemory represents an existing memory loaded from vector store.
type VectorMemory struct {
	ID       string
	Category string
	Text     string
	Scope    string
}

func BuildPrompt(conversation string, existingTenant, existingUser []VectorMemory, hasUser bool) string {
	var b strings.Builder
	b.WriteString(`You are a memory consolidation assistant. Analyze the conversation and extract important memories as categorized entries.

## Rules
- Each memory should be concise, self-contained, and categorized
- Categories are free-form: preference, fact, event, instruction, relationship, skill, goal, etc.
- Scope "tenant" = shared knowledge about the workspace/group
- Scope "user" = personal to the specific user (preferences, habits, personal info)
- If a memory updates or contradicts an existing one, output the NEW version (the system handles dedup)
- Do NOT repeat existing memories that haven't changed
- Only extract genuinely useful information, skip trivial chatter
- Prefer saving facts over actions. e.g. "User likes dark mode" > "I changed the setting"
- Use the permanent field: true for preference/identity/instruction/fact (never decays), false for event/goal/general (decays over time)

`)

	if len(existingTenant) > 0 {
		b.WriteString("## Existing Tenant Memories\n")
		for _, m := range existingTenant {
			fmt.Fprintf(&b, "- [%s] %s\n", m.Category, m.Text)
		}
		b.WriteString("\n")
	}

	if hasUser && len(existingUser) > 0 {
		b.WriteString("## Existing User Memories\n")
		for _, m := range existingUser {
			fmt.Fprintf(&b, "- [%s] %s\n", m.Category, m.Text)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Conversation\n%s\n\n", conversation)
	b.WriteString("Call the save_memories tool with the extracted entries.")
	if !hasUser {
		b.WriteString(" All entries must have scope \"tenant\".")
	}
	return b.String()
}
