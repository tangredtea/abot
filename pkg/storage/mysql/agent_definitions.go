package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentDefinitionStore handles CRUD operations for agent definitions.
type AgentDefinitionStore struct {
	db *gorm.DB
}

func NewAgentDefinitionStore(db *gorm.DB) *AgentDefinitionStore {
	return &AgentDefinitionStore{db: db}
}

// AgentDefinition represents a complete agent configuration.
type AgentDefinition struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Avatar      string                 `json:"avatar"`
	Model       string                 `json:"model"`
	Provider    string                 `json:"provider"`
	Status      string                 `json:"status"`
	Config      map[string]interface{} `json:"config"`
	Channels    []AgentChannel         `json:"channels"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AgentChannel represents a channel configuration for an agent.
type AgentChannel struct {
	AgentID   string                 `json:"agent_id"`
	Channel   string                 `json:"channel"`
	Enabled   bool                   `json:"enabled"`
	Config    map[string]interface{} `json:"config"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// List returns all agent definitions for a tenant.
func (s *AgentDefinitionStore) List(ctx context.Context, tenantID string) ([]*AgentDefinition, error) {
	var models []AgentDefinitionModel
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	agents := make([]*AgentDefinition, len(models))
	for i, m := range models {
		agent, err := s.modelToAgent(&m)
		if err != nil {
			return nil, err
		}
		// Load channels
		channels, _ := s.getChannels(ctx, m.ID)
		agent.Channels = channels
		agents[i] = agent
	}
	return agents, nil
}

// ListAll returns all agent definitions from all tenants.
func (s *AgentDefinitionStore) ListAll(ctx context.Context) ([]*AgentDefinition, error) {
	var models []AgentDefinitionModel
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list all agents: %w", err)
	}

	agents := make([]*AgentDefinition, len(models))
	for i, m := range models {
		agent, err := s.modelToAgent(&m)
		if err != nil {
			return nil, err
		}
		// Load channels
		channels, _ := s.getChannels(ctx, m.ID)
		agent.Channels = channels
		agents[i] = agent
	}
	return agents, nil
}

// Get returns a single agent definition by ID.
func (s *AgentDefinitionStore) Get(ctx context.Context, agentID string) (*AgentDefinition, error) {
	var m AgentDefinitionModel
	if err := s.db.WithContext(ctx).Where("id = ?", agentID).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent: %w", err)
	}

	agent, err := s.modelToAgent(&m)
	if err != nil {
		return nil, err
	}

	// Load channels
	channels, _ := s.getChannels(ctx, agentID)
	agent.Channels = channels

	return agent, nil
}

// Create creates a new agent definition.
func (s *AgentDefinitionStore) Create(ctx context.Context, agent *AgentDefinition) error {
	m, err := s.agentToModel(agent)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&m).Error; err != nil {
			return fmt.Errorf("create agent: %w", err)
		}

		// Create channels
		if len(agent.Channels) > 0 {
			for _, ch := range agent.Channels {
				chModel, err := s.channelToModel(&ch)
				if err != nil {
					return err
				}
				if err := tx.Create(&chModel).Error; err != nil {
					return fmt.Errorf("create channel: %w", err)
				}
			}
		}

		return nil
	})
}

// Update updates an existing agent definition.
func (s *AgentDefinitionStore) Update(ctx context.Context, agent *AgentDefinition) error {
	m, err := s.agentToModel(agent)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Save(&m).Error; err != nil {
			return fmt.Errorf("update agent: %w", err)
		}

		// Update channels - delete old and insert new
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&AgentChannelModel{}).Error; err != nil {
			return fmt.Errorf("delete old channels: %w", err)
		}

		if len(agent.Channels) > 0 {
			for _, ch := range agent.Channels {
				chModel, err := s.channelToModel(&ch)
				if err != nil {
					return err
				}
				if err := tx.Create(&chModel).Error; err != nil {
					return fmt.Errorf("create channel: %w", err)
				}
			}
		}

		return nil
	})
}

// Delete deletes an agent definition and its channels.
func (s *AgentDefinitionStore) Delete(ctx context.Context, agentID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&AgentChannelModel{}).Error; err != nil {
			return fmt.Errorf("delete channels: %w", err)
		}
		if err := tx.Where("id = ?", agentID).Delete(&AgentDefinitionModel{}).Error; err != nil {
			return fmt.Errorf("delete agent: %w", err)
		}
		return nil
	})
}

// getChannels loads all channels for an agent.
func (s *AgentDefinitionStore) getChannels(ctx context.Context, agentID string) ([]AgentChannel, error) {
	var models []AgentChannelModel
	if err := s.db.WithContext(ctx).Where("agent_id = ?", agentID).Find(&models).Error; err != nil {
		return nil, err
	}

	channels := make([]AgentChannel, len(models))
	for i, m := range models {
		ch, err := s.modelToChannel(&m)
		if err != nil {
			return nil, err
		}
		channels[i] = *ch
	}
	return channels, nil
}

// modelToAgent converts database model to domain type.
func (s *AgentDefinitionStore) modelToAgent(m *AgentDefinitionModel) (*AgentDefinition, error) {
	var config map[string]interface{}
	if len(m.Config) > 0 {
		if err := json.Unmarshal(m.Config, &config); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	}

	return &AgentDefinition{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Name:        m.Name,
		Description: m.Description,
		Avatar:      m.Avatar,
		Model:       m.Model,
		Provider:    m.Provider,
		Status:      m.Status,
		Config:      config,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}

// agentToModel converts domain type to database model.
func (s *AgentDefinitionStore) agentToModel(a *AgentDefinition) (*AgentDefinitionModel, error) {
	var configJSON JSON
	if a.Config != nil {
		data, err := json.Marshal(a.Config)
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		configJSON = JSON(data)
	}

	return &AgentDefinitionModel{
		ID:          a.ID,
		TenantID:    a.TenantID,
		Name:        a.Name,
		Description: a.Description,
		Avatar:      a.Avatar,
		Model:       a.Model,
		Provider:    a.Provider,
		Status:      a.Status,
		Config:      configJSON,
	}, nil
}

// modelToChannel converts database model to domain type.
func (s *AgentDefinitionStore) modelToChannel(m *AgentChannelModel) (*AgentChannel, error) {
	var config map[string]interface{}
	if len(m.Config) > 0 {
		if err := json.Unmarshal(m.Config, &config); err != nil {
			return nil, fmt.Errorf("unmarshal channel config: %w", err)
		}
	}

	return &AgentChannel{
		AgentID:   m.AgentID,
		Channel:   m.Channel,
		Enabled:   m.Enabled,
		Config:    config,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

// channelToModel converts domain type to database model.
func (s *AgentDefinitionStore) channelToModel(c *AgentChannel) (*AgentChannelModel, error) {
	var configJSON JSON
	if c.Config != nil {
		data, err := json.Marshal(c.Config)
		if err != nil {
			return nil, fmt.Errorf("marshal channel config: %w", err)
		}
		configJSON = JSON(data)
	}

	return &AgentChannelModel{
		AgentID: c.AgentID,
		Channel: c.Channel,
		Enabled: c.Enabled,
		Config:  configJSON,
	}, nil
}
