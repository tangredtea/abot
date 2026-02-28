package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// --- subagent (synchronous sub-agent tool) ---

type subagentArgs struct {
	Task    string `json:"task" jsonschema:"Task for the subagent to complete"`
	AgentID string `json:"agent_id,omitempty" jsonschema:"Target agent ID (optional, uses default if empty)"`
}

type subagentResult struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// newSubagent creates a synchronous sub-agent tool.
// Unlike spawn, subagent blocks until the sub-agent completes and returns
// the result, allowing the LLM to use the output within the same turn.
func newSubagent(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "subagent",
		Description: "Execute a subtask synchronously via a subagent and return the result. Use this when you need the result before continuing.",
	}, func(ctx tool.Context, args subagentArgs) (subagentResult, error) {
		if deps.Subagent == nil {
			return subagentResult{Error: "subagent not configured"}, nil
		}
		if args.Task == "" {
			return subagentResult{Error: "task is required"}, nil
		}

		ch := stateStr(ctx, "channel")
		chatID := stateStr(ctx, "chat_id")
		tenantID := stateStr(ctx, "tenant_id")
		userID := stateStr(ctx, "user_id")

		result, err := deps.Subagent.SpawnSync(ctx, args.Task, args.AgentID, ch, chatID, tenantID, userID)
		if err != nil {
			return subagentResult{Error: fmt.Sprintf("subagent failed: %v", err)}, nil
		}
		return subagentResult{Result: result}, nil
	})
	return t
}
