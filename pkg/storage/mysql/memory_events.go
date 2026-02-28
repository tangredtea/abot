package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// MemoryEventStoreMySQL implements types.MemoryEventStore.
type MemoryEventStoreMySQL struct {
	db *gorm.DB
}

func NewMemoryEventStore(db *gorm.DB) *MemoryEventStoreMySQL {
	return &MemoryEventStoreMySQL{db: db}
}

func (s *MemoryEventStoreMySQL) Add(ctx context.Context, event *types.MemoryEvent) error {
	m := MemoryEventModel{
		TenantID:  event.TenantID,
		UserID:    event.UserID,
		Category:  event.Category,
		Summary:   event.Summary,
		CreatedAt: event.CreatedAt,
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return s.db.WithContext(ctx).Create(&m).Error
}

func (s *MemoryEventStoreMySQL) List(ctx context.Context, tenantID, userID string, from, to time.Time, limit int) ([]*types.MemoryEvent, error) {
	q := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at <= ?", to)
	}
	if limit <= 0 {
		limit = 50
	}
	var models []MemoryEventModel
	if err := q.Order("created_at DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*types.MemoryEvent, len(models))
	for i := range models {
		out[i] = &types.MemoryEvent{
			ID:        models[i].ID,
			TenantID:  models[i].TenantID,
			UserID:    models[i].UserID,
			Category:  models[i].Category,
			Summary:   models[i].Summary,
			CreatedAt: models[i].CreatedAt,
		}
	}
	return out, nil
}

var _ types.MemoryEventStore = (*MemoryEventStoreMySQL)(nil)
