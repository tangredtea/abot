package mysql

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"abot/pkg/types"
)

type AllowlistStore struct {
	db *gorm.DB
}

func NewAllowlistStore(db *gorm.DB) *AllowlistStore {
	return &AllowlistStore{db: db}
}

func (s *AllowlistStore) GetEntry(ctx context.Context, tenantID, chatID string) (*types.AllowlistEntry, error) {
	var m AllowlistModel
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND chat_id = ?", tenantID, chatID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entry types.AllowlistEntry
	_ = json.Unmarshal([]byte(m.Entry), &entry)
	return &entry, nil
}

func (s *AllowlistStore) SetEntry(ctx context.Context, tenantID, chatID string, entry *types.AllowlistEntry) error {
	data, _ := json.Marshal(entry)
	m := &AllowlistModel{
		TenantID: tenantID,
		ChatID:   chatID,
		Entry:    JSON(data),
	}
	return s.db.WithContext(ctx).Save(m).Error
}

func (s *AllowlistStore) DeleteEntry(ctx context.Context, tenantID, chatID string) error {
	return s.db.WithContext(ctx).Where("tenant_id = ? AND chat_id = ?", tenantID, chatID).Delete(&AllowlistModel{}).Error
}

func (s *AllowlistStore) ListEntries(ctx context.Context, tenantID string) (map[string]types.AllowlistEntry, error) {
	var models []AllowlistModel
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]types.AllowlistEntry)
	for _, m := range models {
		var entry types.AllowlistEntry
		_ = json.Unmarshal([]byte(m.Entry), &entry)
		result[m.ChatID] = entry
	}
	return result, nil
}

var _ types.AllowlistStore = (*AllowlistStore)(nil)
