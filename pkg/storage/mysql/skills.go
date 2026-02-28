// Package mysql implements persistent storage for tenants, workspaces, skills,
// agents, and scheduled jobs using MySQL via GORM.
package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"abot/pkg/types"
)

// SkillRegistryStoreMySQL implements types.SkillRegistryStore.
type SkillRegistryStoreMySQL struct {
	db *gorm.DB
}

func NewSkillRegistryStore(db *gorm.DB) *SkillRegistryStoreMySQL {
	return &SkillRegistryStoreMySQL{db: db}
}

func (s *SkillRegistryStoreMySQL) Get(ctx context.Context, name string) (*types.SkillRecord, error) {
	var m SkillRecordModel
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&m).Error; err != nil {
		return nil, fmt.Errorf("SkillRegistryStore.Get(%s): %w", name, err)
	}
	return skillRecordFromModel(&m), nil
}

func (s *SkillRegistryStoreMySQL) GetByID(ctx context.Context, id int64) (*types.SkillRecord, error) {
	var m SkillRecordModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, fmt.Errorf("SkillRegistryStore.GetByID(%d): %w", id, err)
	}
	return skillRecordFromModel(&m), nil
}

func (s *SkillRegistryStoreMySQL) GetByIDs(ctx context.Context, ids []int64) ([]*types.SkillRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var models []SkillRecordModel
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("SkillRegistryStore.GetByIDs: %w", err)
	}
	out := make([]*types.SkillRecord, len(models))
	for i := range models {
		out[i] = skillRecordFromModel(&models[i])
	}
	return out, nil
}

func (s *SkillRegistryStoreMySQL) List(ctx context.Context, opts types.SkillListOpts) ([]*types.SkillRecord, error) {
	q := s.db.WithContext(ctx)
	if opts.Tier != "" {
		q = q.Where("tier = ?", string(opts.Tier))
	}
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}
	var models []SkillRecordModel
	if err := q.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("SkillRegistryStore.List: %w", err)
	}
	out := make([]*types.SkillRecord, len(models))
	for i := range models {
		out[i] = skillRecordFromModel(&models[i])
	}
	return out, nil
}

func (s *SkillRegistryStoreMySQL) Put(ctx context.Context, record *types.SkillRecord) error {
	m := skillRecordToModel(record)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "version", "object_path", "tier", "always_load", "status", "metadata", "updated_at"}),
	}).Create(&m).Error; err != nil {
		return fmt.Errorf("SkillRegistryStore.Put(%s): %w", record.Name, err)
	}
	return nil
}

func (s *SkillRegistryStoreMySQL) Delete(ctx context.Context, name string) error {
	if err := s.db.WithContext(ctx).Where("name = ?", name).Delete(&SkillRecordModel{}).Error; err != nil {
		return fmt.Errorf("SkillRegistryStore.Delete(%s): %w", name, err)
	}
	return nil
}

// --- converters ---

func skillRecordToModel(r *types.SkillRecord) *SkillRecordModel {
	meta, _ := json.Marshal(r.Metadata)
	return &SkillRecordModel{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Version:     r.Version,
		ObjectPath:  r.ObjectPath,
		Tier:        string(r.Tier),
		AlwaysLoad:  r.AlwaysLoad,
		Status:      r.Status,
		Metadata:    JSON(meta),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func skillRecordFromModel(m *SkillRecordModel) *types.SkillRecord {
	var meta map[string]any
	_ = json.Unmarshal([]byte(m.Metadata), &meta)
	return &types.SkillRecord{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Version:     m.Version,
		ObjectPath:  m.ObjectPath,
		Tier:        types.SkillTier(m.Tier),
		AlwaysLoad:  m.AlwaysLoad,
		Status:      m.Status,
		Metadata:    meta,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ types.SkillRegistryStore = (*SkillRegistryStoreMySQL)(nil)
