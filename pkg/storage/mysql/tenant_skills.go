package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// TenantSkillStoreMySQL implements types.TenantSkillStore.
type TenantSkillStoreMySQL struct {
	db *gorm.DB
}

func NewTenantSkillStore(db *gorm.DB) *TenantSkillStoreMySQL {
	return &TenantSkillStoreMySQL{db: db}
}

func (s *TenantSkillStoreMySQL) Install(ctx context.Context, ts *types.TenantSkill) error {
	m := tenantSkillToModel(ts)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("TenantSkillStore.Install(%s,%d): %w", ts.TenantID, ts.SkillID, err)
	}
	return nil
}

func (s *TenantSkillStoreMySQL) Uninstall(ctx context.Context, tenantID string, skillID int64) error {
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND skill_id = ?", tenantID, skillID).
		Delete(&TenantSkillModel{}).Error; err != nil {
		return fmt.Errorf("TenantSkillStore.Uninstall(%s,%d): %w", tenantID, skillID, err)
	}
	return nil
}

func (s *TenantSkillStoreMySQL) ListInstalled(ctx context.Context, tenantID string) ([]*types.TenantSkill, error) {
	var models []TenantSkillModel
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("priority ASC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("TenantSkillStore.ListInstalled(%s): %w", tenantID, err)
	}
	out := make([]*types.TenantSkill, len(models))
	for i := range models {
		out[i] = tenantSkillFromModel(&models[i])
	}
	return out, nil
}

func (s *TenantSkillStoreMySQL) UpdateConfig(ctx context.Context, tenantID string, skillID int64, config map[string]any) error {
	cfg, _ := json.Marshal(config)
	if err := s.db.WithContext(ctx).
		Model(&TenantSkillModel{}).
		Where("tenant_id = ? AND skill_id = ?", tenantID, skillID).
		Update("config", JSON(cfg)).Error; err != nil {
		return fmt.Errorf("TenantSkillStore.UpdateConfig(%s,%d): %w", tenantID, skillID, err)
	}
	return nil
}

// --- converters ---

func tenantSkillToModel(ts *types.TenantSkill) *TenantSkillModel {
	cfg, _ := json.Marshal(ts.Config)
	m := TenantSkillModel{
		TenantID:    ts.TenantID,
		SkillID:     ts.SkillID,
		Config:      JSON(cfg),
		Priority:    ts.Priority,
		InstalledAt: ts.InstalledAt,
	}
	if ts.AlwaysLoad != nil {
		m.AlwaysLoad = sql.NullBool{Bool: *ts.AlwaysLoad, Valid: true}
	}
	if m.InstalledAt.IsZero() {
		m.InstalledAt = time.Now()
	}
	return &m
}

func tenantSkillFromModel(m *TenantSkillModel) *types.TenantSkill {
	var cfg map[string]any
	_ = json.Unmarshal([]byte(m.Config), &cfg)
	ts := &types.TenantSkill{
		TenantID:    m.TenantID,
		SkillID:     m.SkillID,
		Config:      cfg,
		Priority:    m.Priority,
		InstalledAt: m.InstalledAt,
	}
	if m.AlwaysLoad.Valid {
		v := m.AlwaysLoad.Bool
		ts.AlwaysLoad = &v
	}
	return ts
}

var _ types.TenantSkillStore = (*TenantSkillStoreMySQL)(nil)
