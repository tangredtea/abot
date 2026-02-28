package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"abot/pkg/types"
)

// AgentStoreMySQL implements types.AgentStore.
type AgentStoreMySQL struct {
	db *gorm.DB
}

func NewAgentStore(db *gorm.DB) *AgentStoreMySQL {
	return &AgentStoreMySQL{db: db}
}

// WithTx returns a copy that operates within the given transaction.
func (s *AgentStoreMySQL) WithTx(tx *gorm.DB) *AgentStoreMySQL {
	return &AgentStoreMySQL{db: tx}
}

func (s *AgentStoreMySQL) Get(ctx context.Context, agentID string) (*types.AgentRoute, error) {
	var m AgentRouteModel
	if err := s.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&m).Error; err != nil {
		return nil, fmt.Errorf("AgentStore.Get(%s): %w", agentID, err)
	}
	return agentRouteFromModel(&m), nil
}

func (s *AgentStoreMySQL) List(ctx context.Context) ([]*types.AgentRoute, error) {
	var models []AgentRouteModel
	if err := s.db.WithContext(ctx).Order("priority ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("AgentStore.List: %w", err)
	}
	out := make([]*types.AgentRoute, len(models))
	for i := range models {
		out[i] = agentRouteFromModel(&models[i])
	}
	return out, nil
}

func (s *AgentStoreMySQL) Put(ctx context.Context, route *types.AgentRoute) error {
	m := agentRouteToModel(route)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&m).Error; err != nil {
		return fmt.Errorf("AgentStore.Put(%s): %w", route.AgentID, err)
	}
	return nil
}

func (s *AgentStoreMySQL) Delete(ctx context.Context, agentID string) error {
	if err := s.db.WithContext(ctx).Where("agent_id = ?", agentID).Delete(&AgentRouteModel{}).Error; err != nil {
		return fmt.Errorf("AgentStore.Delete(%s): %w", agentID, err)
	}
	return nil
}

// --- converters ---

func agentRouteToModel(r *types.AgentRoute) *AgentRouteModel {
	m := &AgentRouteModel{
		AgentID:   r.AgentID,
		Channel:   r.Channel,
		ChatID:    r.ChatID,
		AccountID: r.AccountID,
		GuildID:   r.GuildID,
		TeamID:    r.TeamID,
		Priority:  r.Priority,
		IsDefault: r.IsDefault,
	}
	if r.Peer != nil {
		m.PeerKind = r.Peer.Kind
		m.PeerID = r.Peer.ID
	}
	return m
}

func agentRouteFromModel(m *AgentRouteModel) *types.AgentRoute {
	r := &types.AgentRoute{
		AgentID:   m.AgentID,
		Channel:   m.Channel,
		ChatID:    m.ChatID,
		AccountID: m.AccountID,
		GuildID:   m.GuildID,
		TeamID:    m.TeamID,
		Priority:  m.Priority,
		IsDefault: m.IsDefault,
	}
	if m.PeerKind != "" || m.PeerID != "" {
		r.Peer = &types.PeerMatch{Kind: m.PeerKind, ID: m.PeerID}
	}
	return r
}

var _ types.AgentStore = (*AgentStoreMySQL)(nil)
