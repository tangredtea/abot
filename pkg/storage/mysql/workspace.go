package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"abot/pkg/types"
)

// WorkspaceDocStore implements types.WorkspaceStore using GORM.
type WorkspaceDocStore struct {
	db *gorm.DB
}

func NewWorkspaceDocStore(db *gorm.DB) *WorkspaceDocStore {
	return &WorkspaceDocStore{db: db}
}

// WithTx returns a copy that operates within the given transaction.
func (s *WorkspaceDocStore) WithTx(tx *gorm.DB) *WorkspaceDocStore {
	return &WorkspaceDocStore{db: tx}
}

func (s *WorkspaceDocStore) Get(ctx context.Context, tenantID, docType string) (*types.WorkspaceDoc, error) {
	var m WorkspaceDocModel
	err := s.db.WithContext(ctx).Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("tenant_id = ? AND doc_type = ?", tenantID, docType).
		First(&m).Error
	if err != nil {
		return nil, fmt.Errorf("WorkspaceDocStore.Get(%s,%s): %w", tenantID, docType, err)
	}
	return workspaceDocFromModel(&m), nil
}

func (s *WorkspaceDocStore) Put(ctx context.Context, doc *types.WorkspaceDoc) error {
	m := workspaceDocToModel(doc)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&m).Error; err != nil {
		return fmt.Errorf("WorkspaceDocStore.Put(%s,%s): %w", doc.TenantID, doc.DocType, err)
	}
	return nil
}

func (s *WorkspaceDocStore) List(ctx context.Context, tenantID string) ([]*types.WorkspaceDoc, error) {
	var models []WorkspaceDocModel
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("WorkspaceDocStore.List(%s): %w", tenantID, err)
	}
	out := make([]*types.WorkspaceDoc, len(models))
	for i := range models {
		out[i] = workspaceDocFromModel(&models[i])
	}
	return out, nil
}

func (s *WorkspaceDocStore) Delete(ctx context.Context, tenantID, docType string) error {
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND doc_type = ?", tenantID, docType).
		Delete(&WorkspaceDocModel{}).Error; err != nil {
		return fmt.Errorf("WorkspaceDocStore.Delete(%s,%s): %w", tenantID, docType, err)
	}
	return nil
}

func workspaceDocToModel(d *types.WorkspaceDoc) *WorkspaceDocModel {
	return &WorkspaceDocModel{
		TenantID:  d.TenantID,
		DocType:   d.DocType,
		Content:   d.Content,
		Version:   d.Version,
		UpdatedAt: d.UpdatedAt,
	}
}

func workspaceDocFromModel(m *WorkspaceDocModel) *types.WorkspaceDoc {
	return &types.WorkspaceDoc{
		TenantID:  m.TenantID,
		DocType:   m.DocType,
		Content:   m.Content,
		Version:   m.Version,
		UpdatedAt: m.UpdatedAt,
	}
}

var _ types.WorkspaceStore = (*WorkspaceDocStore)(nil)
