// Package scheduler implements cron job scheduling and heartbeat services.
// Jobs are persisted via SchedulerStore and triggered via MessageBus.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"abot/pkg/types"
)

// CronService manages scheduled jobs across all tenants.
// On Start it loads persisted jobs, computes next-run times,
// and enters a loop that sleeps until the earliest due job.
type CronService struct {
	store  types.SchedulerStore
	bus    types.MessageBus
	jobs   map[string]*types.CronJob
	mu     sync.RWMutex
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
	parser cron.Parser
	logger *slog.Logger
}

// New creates a CronService. Call Start to begin scheduling.
func New(store types.SchedulerStore, bus types.MessageBus, logger *slog.Logger) *CronService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CronService{
		store:  store,
		bus:    bus,
		jobs:   make(map[string]*types.CronJob),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		logger: logger,
	}
}

// Start loads persisted jobs and begins the scheduling loop.
// Pass tenantID="" to load jobs for all tenants.
func (s *CronService) Start(ctx context.Context) error {
	jobs, err := s.store.ListJobs(ctx, "")
	if err != nil {
		return fmt.Errorf("scheduler: load jobs: %w", err)
	}

	s.mu.Lock()
	now := time.Now()
	for _, j := range jobs {
		if j.Enabled && j.State.NextRunAt.IsZero() {
			j.State.NextRunAt = s.ComputeNextRun(j, now)
		}
		s.jobs[j.ID] = j
	}
	s.mu.Unlock()

	ctx, s.cancel = context.WithCancel(ctx)
	go s.loop(ctx)

	s.logger.Info("scheduler started", "jobs", len(jobs))
	return nil
}

// Stop cancels the scheduling loop and waits for it to exit.
func (s *CronService) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
	return nil
}

// AddJob persists and schedules a new job.
func (s *CronService) AddJob(ctx context.Context, job *types.CronJob) error {
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.State.NextRunAt.IsZero() {
		job.State.NextRunAt = s.ComputeNextRun(job, now)
	}
	if err := s.store.SaveJob(ctx, job); err != nil {
		return fmt.Errorf("scheduler: save job: %w", err)
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	s.signal()
	return nil
}

// RemoveJob deletes a job from store and memory.
func (s *CronService) RemoveJob(ctx context.Context, jobID string) error {
	if err := s.store.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("scheduler: delete job: %w", err)
	}
	s.mu.Lock()
	delete(s.jobs, jobID)
	s.mu.Unlock()
	s.signal()
	return nil
}

// EnableJob toggles a job's enabled state.
func (s *CronService) EnableJob(ctx context.Context, jobID string, enabled bool) error {
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("scheduler: job %q not found", jobID)
	}
	job.Enabled = enabled
	if enabled {
		job.State.NextRunAt = s.ComputeNextRun(job, time.Now())
	}
	s.mu.Unlock()

	if err := s.store.SaveJob(ctx, job); err != nil {
		return fmt.Errorf("scheduler: save job: %w", err)
	}
	s.signal()
	return nil
}

// ListJobs returns all jobs for a tenant, or all jobs if tenantID is empty.
func (s *CronService) ListJobs(tenantID string) []*types.CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*types.CronJob
	for _, j := range s.jobs {
		if tenantID == "" || j.TenantID == tenantID {
			out = append(out, j)
		}
	}
	return out
}

// signal wakes the scheduling loop to recompute next-run times.
func (s *CronService) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// loop is the main scheduling goroutine.
func (s *CronService) loop(ctx context.Context) {
	defer close(s.done)
	for {
		next, ok := s.earliestRun()
		var timer *time.Timer
		if ok {
			timer = time.NewTimer(max(time.Until(next), 0))
		} else {
			// No jobs scheduled; park until woken.
			timer = time.NewTimer(24 * time.Hour)
		}

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.wake:
			timer.Stop()
			continue
		case <-timer.C:
			s.fireDueJobs(ctx)
		}
	}
}

// earliestRun returns the soonest NextRunAt among enabled jobs.
func (s *CronService) earliestRun() (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var earliest time.Time
	found := false
	for _, j := range s.jobs {
		if !j.Enabled || j.State.NextRunAt.IsZero() {
			continue
		}
		if !found || j.State.NextRunAt.Before(earliest) {
			earliest = j.State.NextRunAt
			found = true
		}
	}
	return earliest, found
}

// fireDueJobs fires all jobs whose NextRunAt <= now.
func (s *CronService) fireDueJobs(ctx context.Context) {
	now := time.Now()
	s.mu.Lock()
	var due []*types.CronJob
	for _, j := range s.jobs {
		if j.Enabled && !j.State.NextRunAt.IsZero() && !j.State.NextRunAt.After(now) {
			due = append(due, j)
		}
	}
	s.mu.Unlock()

	for _, j := range due {
		s.fireJob(ctx, j, now)
	}
}

// fireJob publishes a job's message to the bus and updates state.
func (s *CronService) fireJob(ctx context.Context, job *types.CronJob, now time.Time) {
	msg := types.InboundMessage{
		Channel:   job.Channel,
		TenantID:  job.TenantID,
		UserID:    job.UserID,
		ChatID:    job.ChatID,
		Content:   job.Message,
		Metadata:  map[string]string{"cron_job_id": job.ID, "cron_job_name": job.Name},
		Timestamp: now,
	}

	err := s.bus.PublishInbound(ctx, msg)

	s.mu.Lock()
	job.State.LastRunAt = now
	if err != nil {
		job.State.LastStatus = "error"
		job.State.LastError = err.Error()
		s.logger.Error("scheduler: fire job", "job", job.ID, "err", err)
	} else {
		job.State.LastStatus = "ok"
		job.State.LastError = ""
	}

	if job.DeleteAfterRun && job.Schedule.Kind == types.ScheduleAt {
		delete(s.jobs, job.ID)
		s.mu.Unlock()
		_ = s.store.DeleteJob(ctx, job.ID)
		return
	}

	job.State.NextRunAt = s.ComputeNextRun(job, now)
	s.mu.Unlock()

	_ = s.store.UpdateJobState(ctx, job.ID, &job.State)
}

// ComputeNextRun calculates the next execution time for a job.
func (s *CronService) ComputeNextRun(job *types.CronJob, after time.Time) time.Time {
	switch job.Schedule.Kind {
	case types.ScheduleAt:
		if job.Schedule.AtTime.After(after) {
			return job.Schedule.AtTime
		}
		return time.Time{} // already past

	case types.ScheduleEvery:
		if job.State.LastRunAt.IsZero() {
			return after.Add(job.Schedule.Interval)
		}
		next := job.State.LastRunAt.Add(job.Schedule.Interval)
		if next.Before(after) {
			return after.Add(job.Schedule.Interval)
		}
		return next

	case types.ScheduleCron:
		sched, err := s.parseCronExpr(job.Schedule.Expr, job.Schedule.Timezone)
		if err != nil {
			s.logger.Error("scheduler: bad cron expr", "job", job.ID, "expr", job.Schedule.Expr, "err", err)
			return time.Time{}
		}
		return sched.Next(after)
	}
	return time.Time{}
}

// parseCronExpr parses a cron expression with optional timezone.
func (s *CronService) parseCronExpr(expr, tz string) (cron.Schedule, error) {
	loc := time.UTC
	if tz != "" {
		var err error
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("bad timezone %q: %w", tz, err)
		}
	}
	sched, err := s.parser.Parse(expr)
	if err != nil {
		return nil, err
	}
	// Wrap with timezone-aware schedule.
	return &tzSchedule{inner: sched, loc: loc}, nil
}

// tzSchedule wraps a cron.Schedule to evaluate Next in a specific timezone.
type tzSchedule struct {
	inner cron.Schedule
	loc   *time.Location
}

func (t *tzSchedule) Next(now time.Time) time.Time {
	return t.inner.Next(now.In(t.loc))
}

// SchedulerStatus describes a summary of the scheduler's running state.
type SchedulerStatus struct {
	Running      bool       `json:"running"`
	TotalJobs    int        `json:"total_jobs"`
	EnabledJobs  int        `json:"enabled_jobs"`
	NextWakeAt   *time.Time `json:"next_wake_at,omitempty"`
}

// Status returns a snapshot of the scheduler's current running state.
func (s *CronService) Status() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st := SchedulerStatus{
		Running:   s.cancel != nil,
		TotalJobs: len(s.jobs),
	}

	var earliest time.Time
	found := false
	for _, j := range s.jobs {
		if j.Enabled {
			st.EnabledJobs++
			if !j.State.NextRunAt.IsZero() && (!found || j.State.NextRunAt.Before(earliest)) {
				earliest = j.State.NextRunAt
				found = true
			}
		}
	}
	if found {
		st.NextWakeAt = &earliest
	}

	return st
}
