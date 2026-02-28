---
name: cron
description: Schedule tasks to run at specific times, intervals, or cron expressions. Use when the user asks to set reminders, schedule recurring tasks, or automate periodic actions.
always: true
---

# Cron

Manage scheduled tasks using the `cron` tool.

## Actions

- **add** — Create a new scheduled task
- **list** — Show all scheduled tasks for the current tenant
- **remove** — Delete a scheduled task by ID
- **enable/disable** — Toggle a task on or off

## Schedule Modes

### One-time (`at`)
Run once at a specific time.
```
cron add --name "deploy-reminder" --at "2025-03-01T10:00:00Z" --message "Time to deploy v2.1"
```

### Recurring (`every`)
Run at fixed intervals.
```
cron add --name "health-check" --every "30m" --message "Run health check on all services"
```

### Cron expression (`cron`)
Standard cron syntax with timezone support.
```
cron add --name "daily-report" --cron "0 9 * * *" --timezone "Asia/Shanghai" --message "Generate daily report"
```

## Notes

- All tasks are scoped to the current tenant
- Tasks persist across restarts (stored in MySQL)
- One-time tasks can auto-delete after execution with `--delete-after-run`
- Triggered tasks arrive as normal messages to the agent
