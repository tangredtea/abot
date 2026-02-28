package tools

import (
	"fmt"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type spawnArgs struct {
	Task    string `json:"task" jsonschema:"Task description for the subagent"`
	AgentID string `json:"agent_id,omitempty" jsonschema:"Target agent ID (optional)"`
}

type spawnResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newSpawn(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "spawn",
		Description: "Spawn a background subtask for another agent to handle asynchronously.",
	}, func(ctx tool.Context, args spawnArgs) (spawnResult, error) {
		if args.Task == "" {
			return spawnResult{Error: "task description is required"}, nil
		}

		// Read origin context from session state
		ch := stateStr(ctx, "channel")
		chatID := stateStr(ctx, "chat_id")
		tenantID := stateStr(ctx, "tenant_id")

		// Publish as inbound message for the target agent
		msg := types.InboundMessage{
			Channel:  ch,
			TenantID: tenantID,
			ChatID:   chatID,
			Content:  args.Task,
			Metadata: map[string]string{
				"type":     "spawn",
				"agent_id": args.AgentID,
			},
		}
		if err := deps.Bus.PublishInbound(ctx, msg); err != nil {
			return spawnResult{Error: fmt.Sprintf("spawn failed: %v", err)}, nil
		}

		label := "subtask"
		if args.AgentID != "" {
			label = fmt.Sprintf("subtask for agent %s", args.AgentID)
		}
		return spawnResult{Result: fmt.Sprintf("%s dispatched", label)}, nil
	})
	return t
}
