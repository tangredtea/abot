package mysql

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// SchedulerStoreMySQL implements types.SchedulerStore.
type SchedulerStoreMySQL struct {
	db *gorm.DB
}

func NewSchedulerStore(db *gorm.DB) *SchedulerStoreMySQL {
	return &SchedulerStoreMySQL{db: db}
}

func (s *SchedulerStoreMySQL) SaveJob(ctx context.Context, job *types.CronJob) error {
	m := cronJobToModel(job)
	return s.db.WithContext(ctx).Save(&m).Error
}

func (s *SchedulerStoreMySQL) ListJobs(ctx context.Context, tenantID string) ([]*types.CronJob, error) {
	var models []CronJobModel
	q := s.db.WithContext(ctx)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*types.CronJob, len(models))
	for i := range models {
		out[i] = cronJobFromModel(&models[i])
	}
	return out, nil
}

func (s *SchedulerStoreMySQL) DeleteJob(ctx context.Context, jobID string) error {
	return s.db.WithContext(ctx).Where("id = ?", jobID).Delete(&CronJobModel{}).Error
}

func (s *SchedulerStoreMySQL) UpdateJobState(ctx context.Context, jobID string, state *types.CronJobState) error {
	raw, _ := json.Marshal(state)
	return s.db.WithContext(ctx).
		Model(&CronJobModel{}).
		Where("id = ?", jobID).
		Update("state", JSON(raw)).Error
}

func (s *SchedulerStoreMySQL) LogExecution(ctx context.Context, log *types.CronJobLog) error {
	m := &CronJobLogModel{
		JobID:      log.JobID,
		RunAt:      log.RunAt,
		DurationMs: log.DurationMs,
		Status:     log.Status,
		Error:      log.Error,
	}
	return s.db.WithContext(ctx).Create(m).Error
}

func (s *SchedulerStoreMySQL) ListLogs(ctx context.Context, jobID string, limit int) ([]*types.CronJobLog, error) {
	var models []CronJobLogModel
	q := s.db.WithContext(ctx).Where("job_id = ?", jobID).Order("run_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*types.CronJobLog, len(models))
	for i := range models {
		out[i] = &types.CronJobLog{
			ID:         models[i].ID,
			JobID:      models[i].JobID,
			RunAt:      models[i].RunAt,
			DurationMs: models[i].DurationMs,
			Status:     models[i].Status,
			Error:      models[i].Error,
		}
	}
	return out, nil
}

// --- converters ---

func cronJobToModel(j *types.CronJob) *CronJobModel {
	sched, _ := json.Marshal(j.Schedule)
	state, _ := json.Marshal(j.State)
	return &CronJobModel{
		ID:             j.ID,
		TenantID:       j.TenantID,
		UserID:         j.UserID,
		Name:           j.Name,
		Enabled:        j.Enabled,
		Schedule:       JSON(sched),
		Message:        j.Message,
		Channel:        j.Channel,
		ChatID:         j.ChatID,
		DeleteAfterRun: j.DeleteAfterRun,
		State:          JSON(state),
		CreatedAt:      j.CreatedAt,
	}
}

func cronJobFromModel(m *CronJobModel) *types.CronJob {
	var sched types.CronSchedule
	_ = json.Unmarshal([]byte(m.Schedule), &sched)
	var state types.CronJobState
	_ = json.Unmarshal([]byte(m.State), &state)
	return &types.CronJob{
		ID:             m.ID,
		TenantID:       m.TenantID,
		UserID:         m.UserID,
		Name:           m.Name,
		Enabled:        m.Enabled,
		Schedule:       sched,
		Message:        m.Message,
		Channel:        m.Channel,
		ChatID:         m.ChatID,
		DeleteAfterRun: m.DeleteAfterRun,
		State:          state,
		CreatedAt:      m.CreatedAt,
	}
}

var _ types.SchedulerStore = (*SchedulerStoreMySQL)(nil)
