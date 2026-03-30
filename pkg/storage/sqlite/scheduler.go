package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"abot/pkg/types"
)

type SchedulerStore struct {
	db *sql.DB
}

func NewSchedulerStore(db *sql.DB) *SchedulerStore {
	return &SchedulerStore{db: db}
}

func (s *SchedulerStore) SaveJob(ctx context.Context, job *types.CronJob) error {
	sched, _ := json.Marshal(job.Schedule)
	state, _ := json.Marshal(job.State)
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO cron_jobs (id, tenant_id, user_id, name, enabled, schedule, message, channel, chat_id, delete_after_run, state, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.TenantID, job.UserID, job.Name, job.Enabled, sched, job.Message, job.Channel, job.ChatID, job.DeleteAfterRun, state, job.CreatedAt)
	return err
}

func (s *SchedulerStore) ListJobs(ctx context.Context, tenantID string) ([]*types.CronJob, error) {
	query := "SELECT id, tenant_id, user_id, name, enabled, schedule, message, channel, chat_id, delete_after_run, state, created_at FROM cron_jobs"
	args := []any{}
	if tenantID != "" {
		query += " WHERE tenant_id = ?"
		args = append(args, tenantID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*types.CronJob
	for rows.Next() {
		var j types.CronJob
		var schedJSON, stateJSON []byte
		if err := rows.Scan(&j.ID, &j.TenantID, &j.UserID, &j.Name, &j.Enabled, &schedJSON, &j.Message, &j.Channel, &j.ChatID, &j.DeleteAfterRun, &stateJSON, &j.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(schedJSON, &j.Schedule)
		_ = json.Unmarshal(stateJSON, &j.State)
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

func (s *SchedulerStore) DeleteJob(ctx context.Context, jobID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM cron_jobs WHERE id = ?", jobID)
	return err
}

func (s *SchedulerStore) UpdateJobState(ctx context.Context, jobID string, state *types.CronJobState) error {
	stateJSON, _ := json.Marshal(state)
	_, err := s.db.ExecContext(ctx, "UPDATE cron_jobs SET state = ? WHERE id = ?", stateJSON, jobID)
	return err
}

func (s *SchedulerStore) LogExecution(ctx context.Context, log *types.CronJobLog) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cron_job_logs (job_id, run_at, duration_ms, status, error)
		VALUES (?, ?, ?, ?, ?)`,
		log.JobID, log.RunAt, log.DurationMs, log.Status, log.Error)
	return err
}

func (s *SchedulerStore) ListLogs(ctx context.Context, jobID string, limit int) ([]*types.CronJobLog, error) {
	query := "SELECT id, job_id, run_at, duration_ms, status, error FROM cron_job_logs WHERE job_id = ? ORDER BY run_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
	}
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, query, jobID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, query, jobID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*types.CronJobLog
	for rows.Next() {
		var l types.CronJobLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.RunAt, &l.DurationMs, &l.Status, &l.Error); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, rows.Err()
}

var _ types.SchedulerStore = (*SchedulerStore)(nil)
