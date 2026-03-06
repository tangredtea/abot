package types

import "time"

// CronScheduleKind defines the scheduling mode.
type CronScheduleKind string

const (
	ScheduleAt    CronScheduleKind = "at"
	ScheduleEvery CronScheduleKind = "every"
	ScheduleCron  CronScheduleKind = "cron"
)

// CronJob represents a scheduled task.
type CronJob struct {
	ID             string
	TenantID       string
	UserID         string
	Name           string
	Enabled        bool
	Schedule       CronSchedule
	Message        string
	Channel        string
	ChatID         string
	DeleteAfterRun bool
	State          CronJobState
	CreatedAt      time.Time
}

// CronSchedule holds the scheduling parameters.
type CronSchedule struct {
	Kind     CronScheduleKind
	AtTime   time.Time
	Interval time.Duration
	Expr     string
	Timezone string
}

// CronJobState tracks execution state.
type CronJobState struct {
	NextRunAt  time.Time
	LastRunAt  time.Time
	LastStatus string
	LastError  string
}

// CronJobLog records a single execution of a cron job.
type CronJobLog struct {
	ID         int64
	JobID      string
	RunAt      time.Time
	DurationMs int64
	Status     string
	Error      string
}
