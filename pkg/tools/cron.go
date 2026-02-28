package tools

import (
	"context"
	"fmt"
	"time"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type cronArgs struct {
	Action   string `json:"action" jsonschema:"required,enum=add|list|remove|enable|disable,description=Action to perform"`
	JobID    string `json:"job_id,omitempty" jsonschema:"description=Job ID returned by add. Required for remove/enable/disable."`
	Name     string `json:"name,omitempty" jsonschema:"description=Human-readable job name for add action."`
	Message  string `json:"message,omitempty" jsonschema:"description=Message content to deliver when the job fires. Required for add."`
	AtSecs   int    `json:"at_seconds,omitempty" jsonschema:"description=One-shot: fire once after this many seconds from now. Example: 300 means fire in 5 minutes."`
	EverySec int    `json:"every_seconds,omitempty" jsonschema:"description=Recurring: fire every N seconds. Example: 60 means fire every 1 minute."`
	CronExpr string `json:"cron_expr,omitempty" jsonschema:"description=Standard 5-field cron expression (minute hour day month weekday). Example: 0 9 * * * means every day at 9:00."`
	Timezone string `json:"timezone,omitempty" jsonschema:"description=IANA timezone for cron_expr. Default UTC. Example: Asia/Shanghai."`
}

type cronResult struct {
	Result string         `json:"result,omitempty"`
	Jobs   []cronJobEntry `json:"jobs,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type cronJobEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule"`
}

func newCron(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name: "cron",
		Description: `Manage scheduled tasks.

Actions:
- add: Create a job. Requires "message" and one of: "every_seconds" (integer, recurring), "at_seconds" (integer, one-shot delay), or "cron_expr" (string, cron syntax). Optional: "name", "timezone".
- list: List all jobs for current tenant.
- remove: Delete a job by "job_id".
- enable / disable: Toggle a job by "job_id".

Examples:
  {"action":"add","message":"hello","every_seconds":60}         → fire every 1 min
  {"action":"add","message":"report","cron_expr":"0 9 * * *"}   → daily at 09:00
  {"action":"add","message":"remind","at_seconds":300}          → once in 5 min
  {"action":"list"}
  {"action":"remove","job_id":"cron-xxx"}`,
	}, func(ctx tool.Context, args cronArgs) (cronResult, error) {
		tenantID := stateStr(ctx, "tenant_id")

		switch args.Action {
		case "add":
			return cronAdd(ctx, deps, args, tenantID)
		case "list":
			return cronList(ctx, deps, tenantID)
		case "remove":
			return cronRemove(ctx, deps, args.JobID, tenantID)
		case "enable":
			return cronToggle(ctx, deps, args.JobID, true, tenantID)
		case "disable":
			return cronToggle(ctx, deps, args.JobID, false, tenantID)
		default:
			return cronResult{Error: fmt.Sprintf("unknown action %q, use: add, list, remove, enable, disable", args.Action)}, nil
		}
	})
	return t
}

func cronAdd(ctx tool.Context, deps *Deps, args cronArgs, tenantID string) (cronResult, error) {
	if args.Message == "" {
		return cronResult{Error: "message is required for add"}, nil
	}

	var sched types.CronSchedule
	switch {
	case args.AtSecs > 0:
		sched = types.CronSchedule{
			Kind:   types.ScheduleAt,
			AtTime: time.Now().Add(time.Duration(args.AtSecs) * time.Second),
		}
	case args.EverySec > 0:
		sched = types.CronSchedule{
			Kind:     types.ScheduleEvery,
			Interval: time.Duration(args.EverySec) * time.Second,
		}
	case args.CronExpr != "":
		sched = types.CronSchedule{
			Kind:     types.ScheduleCron,
			Expr:     args.CronExpr,
			Timezone: args.Timezone,
		}
	default:
		return cronResult{Error: "specify at_seconds, every_seconds, or cron_expr"}, nil
	}

	// Read channel context from session state
	ch := stateStr(ctx, "channel")
	chatID := stateStr(ctx, "chat_id")
	userID := stateStr(ctx, "user_id")

	job := &types.CronJob{
		ID:             fmt.Sprintf("cron-%d", time.Now().UnixNano()),
		TenantID:       tenantID,
		UserID:         userID,
		Name:           args.Name,
		Enabled:        true,
		Schedule:       sched,
		Message:        args.Message,
		Channel:        ch,
		ChatID:         chatID,
		DeleteAfterRun: sched.Kind == types.ScheduleAt,
		CreatedAt:      time.Now(),
	}

	if deps.CronScheduler != nil {
		if err := deps.CronScheduler.AddJob(ctx, job); err != nil {
			return cronResult{Error: fmt.Sprintf("add failed: %v", err)}, nil
		}
	} else if err := deps.SchedulerStore.SaveJob(ctx, job); err != nil {
		return cronResult{Error: fmt.Sprintf("save failed: %v", err)}, nil
	}
	return cronResult{Result: fmt.Sprintf("job %s created", job.ID)}, nil
}

func cronList(ctx context.Context, deps *Deps, tenantID string) (cronResult, error) {
	var jobs []*types.CronJob
	if deps.CronScheduler != nil {
		jobs = deps.CronScheduler.ListJobs(tenantID)
	} else {
		var err error
		jobs, err = deps.SchedulerStore.ListJobs(ctx, tenantID)
		if err != nil {
			return cronResult{Error: fmt.Sprintf("list failed: %v", err)}, nil
		}
	}
	entries := make([]cronJobEntry, 0, len(jobs))
	for _, j := range jobs {
		desc := string(j.Schedule.Kind)
		switch j.Schedule.Kind {
		case types.ScheduleEvery:
			desc = fmt.Sprintf("every %s", j.Schedule.Interval)
		case types.ScheduleCron:
			desc = j.Schedule.Expr
		case types.ScheduleAt:
			desc = fmt.Sprintf("at %s", j.Schedule.AtTime.Format(time.RFC3339))
		}
		entries = append(entries, cronJobEntry{
			ID:       j.ID,
			Name:     j.Name,
			Enabled:  j.Enabled,
			Schedule: desc,
		})
	}
	return cronResult{Jobs: entries}, nil
}

func cronRemove(ctx context.Context, deps *Deps, jobID, tenantID string) (cronResult, error) {
	if jobID == "" {
		return cronResult{Error: "job_id is required"}, nil
	}
	if !cronJobOwnedByTenant(deps, jobID, tenantID) {
		return cronResult{Error: "job not found"}, nil
	}
	if deps.CronScheduler != nil {
		if err := deps.CronScheduler.RemoveJob(ctx, jobID); err != nil {
			return cronResult{Error: fmt.Sprintf("delete failed: %v", err)}, nil
		}
	} else if err := deps.SchedulerStore.DeleteJob(ctx, jobID); err != nil {
		return cronResult{Error: fmt.Sprintf("delete failed: %v", err)}, nil
	}
	return cronResult{Result: fmt.Sprintf("job %s removed", jobID)}, nil
}

func cronToggle(ctx context.Context, deps *Deps, jobID string, enabled bool, tenantID string) (cronResult, error) {
	if jobID == "" {
		return cronResult{Error: "job_id is required"}, nil
	}
	if !cronJobOwnedByTenant(deps, jobID, tenantID) {
		return cronResult{Error: "job not found"}, nil
	}
	if deps.CronScheduler != nil {
		if err := deps.CronScheduler.EnableJob(ctx, jobID, enabled); err != nil {
			return cronResult{Error: fmt.Sprintf("toggle failed: %v", err)}, nil
		}
	} else {
		// Fallback: direct store update (no in-memory scheduler)
		state := &types.CronJobState{}
		if enabled {
			state.LastStatus = "enabled"
		} else {
			state.LastStatus = "disabled"
		}
		if err := deps.SchedulerStore.UpdateJobState(ctx, jobID, state); err != nil {
			return cronResult{Error: fmt.Sprintf("toggle failed: %v", err)}, nil
		}
	}
	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	return cronResult{Result: fmt.Sprintf("job %s %s", jobID, action)}, nil
}

// cronJobOwnedByTenant checks whether a job belongs to the given tenant.
func cronJobOwnedByTenant(deps *Deps, jobID, tenantID string) bool {
	if deps.CronScheduler == nil {
		return true // no in-memory scheduler, skip check
	}
	for _, j := range deps.CronScheduler.ListJobs(tenantID) {
		if j.ID == jobID {
			return true
		}
	}
	return false
}
