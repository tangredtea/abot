package tools

import (
	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// --- list_tasks (query sub-task status) ---

type listTasksArgs struct {
	TaskID string `json:"task_id,omitempty" jsonschema:"Specific task ID to query (optional, lists all if empty)"`
}

type listTasksResult struct {
	Tasks []types.TaskSummary `json:"tasks,omitempty"`
	Error string              `json:"error,omitempty"`
}

// newListTasks creates a tool for querying sub-task status.
// Allows the LLM to inspect the execution status and results of spawned async sub-tasks.
func newListTasks(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "list_tasks",
		Description: "List spawned subtask status. Query a specific task by ID or list all.",
	}, func(ctx tool.Context, args listTasksArgs) (listTasksResult, error) {
		if deps.Subagent == nil {
			return listTasksResult{Error: "subagent not configured"}, nil
		}

		// Query a single task.
		if args.TaskID != "" {
			status, result, found := deps.Subagent.GetTaskStatus(args.TaskID)
			if !found {
				return listTasksResult{Error: "task not found: " + args.TaskID}, nil
			}
			return listTasksResult{
				Tasks: []types.TaskSummary{{
					ID:     args.TaskID,
					Status: status,
					Task:   result,
				}},
			}, nil
		}

		// List all tasks.
		tasks := deps.Subagent.ListTasks()
		if len(tasks) == 0 {
			return listTasksResult{Tasks: []types.TaskSummary{}}, nil
		}
		return listTasksResult{Tasks: tasks}, nil
	})
	return t
}
