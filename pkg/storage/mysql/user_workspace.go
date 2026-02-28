package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"abot/pkg/types"
)

// UserWorkspaceDocStore implements types.UserWorkspaceStore using GORM.
type UserWorkspaceDocStore struct {
	db *gorm.DB
}

func NewUserWorkspaceDocStore(db *gorm.DB) *UserWorkspaceDocStore {
	return &UserWorkspaceDocStore{db: db}
}

// WithTx returns a copy that operates within the given transaction.
func (s *UserWorkspaceDocStore) WithTx(tx *gorm.DB) *UserWorkspaceDocStore {
	return &UserWorkspaceDocStore{db: tx}
}

func (s *UserWorkspaceDocStore) Get(ctx context.Context, tenantID, userID, docType string) (*types.UserWorkspaceDoc, error) {
	var m UserWorkspaceDocModel
	err := s.db.WithContext(ctx).Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("tenant_id = ? AND user_id = ? AND doc_type = ?", tenantID, userID, docType).
		First(&m).Error
	if err != nil {
		return nil, fmt.Errorf("UserWorkspaceDocStore.Get(%s,%s,%s): %w", tenantID, userID, docType, err)
	}
	return userWorkspaceDocFromModel(&m), nil
}

func (s *UserWorkspaceDocStore) Put(ctx context.Context, doc *types.UserWorkspaceDoc) error {
	m := userWorkspaceDocToModel(doc)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&m).Error; err != nil {
		return fmt.Errorf("UserWorkspaceDocStore.Put(%s,%s,%s): %w", doc.TenantID, doc.UserID, doc.DocType, err)
	}
	return nil
}

func (s *UserWorkspaceDocStore) List(ctx context.Context, tenantID, userID string) ([]*types.UserWorkspaceDoc, error) {
	var models []UserWorkspaceDocModel
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("UserWorkspaceDocStore.List(%s,%s): %w", tenantID, userID, err)
	}
	out := make([]*types.UserWorkspaceDoc, len(models))
	for i := range models {
		out[i] = userWorkspaceDocFromModel(&models[i])
	}
	return out, nil
}

func (s *UserWorkspaceDocStore) Delete(ctx context.Context, tenantID, userID, docType string) error {
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND doc_type = ?", tenantID, userID, docType).
		Delete(&UserWorkspaceDocModel{}).Error; err != nil {
		return fmt.Errorf("UserWorkspaceDocStore.Delete(%s,%s,%s): %w", tenantID, userID, docType, err)
	}
	return nil
}

func userWorkspaceDocToModel(d *types.UserWorkspaceDoc) *UserWorkspaceDocModel {
	return &UserWorkspaceDocModel{
		TenantID:  d.TenantID,
		UserID:    d.UserID,
		DocType:   d.DocType,
		Content:   d.Content,
		Version:   d.Version,
		UpdatedAt: d.UpdatedAt,
	}
}

func userWorkspaceDocFromModel(m *UserWorkspaceDocModel) *types.UserWorkspaceDoc {
	return &types.UserWorkspaceDoc{
		TenantID:  m.TenantID,
		UserID:    m.UserID,
		DocType:   m.DocType,
		Content:   m.Content,
		Version:   m.Version,
		UpdatedAt: m.UpdatedAt,
	}
}

var _ types.UserWorkspaceStore = (*UserWorkspaceDocStore)(nil)
