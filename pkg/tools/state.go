package tools

import "google.golang.org/adk/tool"

// stateStr reads a string value from session state, returns empty string if not found.
func stateStr(ctx tool.Context, key string) string {
	v, err := ctx.State().Get(key)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
