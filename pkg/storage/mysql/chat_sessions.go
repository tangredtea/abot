package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"abot/pkg/types"
)

// ChatSessionStore implements types.ChatSessionStore using GORM.
type ChatSessionStore struct {
	db *gorm.DB
}

func NewChatSessionStore(db *gorm.DB) *ChatSessionStore {
	return &ChatSessionStore{db: db}
}

func (s *ChatSessionStore) Create(ctx context.Context, cs *types.ChatSession) error {
	m := chatSessionToModel(cs)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("ChatSessionStore.Create(%s): %w", cs.ID, err)
	}
	return nil
}

func (s *ChatSessionStore) Get(ctx context.Context, id string) (*types.ChatSession, error) {
	var m ChatSessionModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, fmt.Errorf("ChatSessionStore.Get(%s): %w", id, err)
	}
	return chatSessionFromModel(&m), nil
}

// GetByAccountID fetches a session only if it belongs to the given account.
// This prevents IDOR by including account_id in the SQL WHERE clause.
func (s *ChatSessionStore) GetByAccountID(ctx context.Context, id, accountID string) (*types.ChatSession, error) {
	var m ChatSessionModel
	if err := s.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, accountID).First(&m).Error; err != nil {
		return nil, fmt.Errorf("ChatSessionStore.GetByAccountID(%s,%s): %w", id, accountID, err)
	}
	return chatSessionFromModel(&m), nil
}

func (s *ChatSessionStore) ListByAccount(ctx context.Context, accountID, tenantID string, archived bool) ([]*types.ChatSession, error) {
	var models []ChatSessionModel
	q := s.db.WithContext(ctx).Where("account_id = ? AND tenant_id = ? AND archived = ?", accountID, tenantID, archived)
	if err := q.Order("updated_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("ChatSessionStore.ListByAccount(%s,%s): %w", accountID, tenantID, err)
	}
	out := make([]*types.ChatSession, len(models))
	for i := range models {
		out[i] = chatSessionFromModel(&models[i])
	}
	return out, nil
}

func (s *ChatSessionStore) Update(ctx context.Context, cs *types.ChatSession) error {
	m := chatSessionToModel(cs)
	if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
		return fmt.Errorf("ChatSessionStore.Update(%s): %w", cs.ID, err)
	}
	return nil
}

func (s *ChatSessionStore) Delete(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&ChatSessionModel{}).Error; err != nil {
		return fmt.Errorf("ChatSessionStore.Delete(%s): %w", id, err)
	}
	return nil
}

func chatSessionToModel(cs *types.ChatSession) *ChatSessionModel {
	return &ChatSessionModel{
		ID:         cs.ID,
		TenantID:   cs.TenantID,
		AccountID:  cs.AccountID,
		AgentID:    cs.AgentID,
		Title:      cs.Title,
		SessionKey: cs.SessionKey,
		Pinned:     cs.Pinned,
		Archived:   cs.Archived,
		CreatedAt:  cs.CreatedAt,
		UpdatedAt:  cs.UpdatedAt,
	}
}

func chatSessionFromModel(m *ChatSessionModel) *types.ChatSession {
	return &types.ChatSession{
		ID:         m.ID,
		TenantID:   m.TenantID,
		AccountID:  m.AccountID,
		AgentID:    m.AgentID,
		Title:      m.Title,
		SessionKey: m.SessionKey,
		Pinned:     m.Pinned,
		Archived:   m.Archived,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

var _ types.ChatSessionStore = (*ChatSessionStore)(nil)
