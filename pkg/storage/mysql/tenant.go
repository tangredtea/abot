package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"abot/pkg/types"
)

// TenantStore implements types.TenantStore using GORM.
type TenantStore struct {
	db *gorm.DB
}

func NewTenantStore(db *gorm.DB) *TenantStore {
	return &TenantStore{db: db}
}

// WithTx returns a copy that operates within the given transaction.
func (s *TenantStore) WithTx(tx *gorm.DB) *TenantStore {
	return &TenantStore{db: tx}
}

func (s *TenantStore) Get(ctx context.Context, tenantID string) (*types.Tenant, error) {
	var m TenantModel
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&m).Error; err != nil {
		return nil, fmt.Errorf("TenantStore.Get(%s): %w", tenantID, err)
	}
	return tenantFromModel(&m), nil
}

func (s *TenantStore) Put(ctx context.Context, tenant *types.Tenant) error {
	m := tenantToModel(tenant)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&m).Error; err != nil {
		return fmt.Errorf("TenantStore.Put(%s): %w", tenant.TenantID, err)
	}
	return nil
}

func (s *TenantStore) List(ctx context.Context, groupID string) ([]*types.Tenant, error) {
	var models []TenantModel
	q := s.db.WithContext(ctx)
	if groupID != "" {
		q = q.Where("group_id = ?", groupID)
	}
	if err := q.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("TenantStore.List(%s): %w", groupID, err)
	}
	out := make([]*types.Tenant, len(models))
	for i := range models {
		out[i] = tenantFromModel(&models[i])
	}
	return out, nil
}

func (s *TenantStore) Delete(ctx context.Context, tenantID string) error {
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Delete(&TenantModel{}).Error; err != nil {
		return fmt.Errorf("TenantStore.Delete(%s): %w", tenantID, err)
	}
	return nil
}

// --- converters ---

func tenantToModel(t *types.Tenant) *TenantModel {
	cfg, _ := json.Marshal(t.Config)
	return &TenantModel{
		TenantID:  t.TenantID,
		Name:      t.Name,
		GroupID:   t.GroupID,
		Config:    JSON(cfg),
		CreatedAt: t.CreatedAt,
	}
}

func tenantFromModel(m *TenantModel) *types.Tenant {
	var cfg map[string]any
	_ = json.Unmarshal([]byte(m.Config), &cfg)
	return &types.Tenant{
		TenantID:  m.TenantID,
		Name:      m.Name,
		GroupID:   m.GroupID,
		Config:    cfg,
		CreatedAt: m.CreatedAt,
	}
}

// compile-time check
var _ types.TenantStore = (*TenantStore)(nil)
