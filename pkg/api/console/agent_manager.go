package console

import (
	"context"
	"fmt"
	"log/slog"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"

	"abot/pkg/agent"
	mysqlstore "abot/pkg/storage/mysql"
	"abot/pkg/types"
)

// AgentManager handles dynamic agent lifecycle (create, update, delete).
type AgentManager struct {
	registry            *agent.AgentRegistry
	agentDefStore       *mysqlstore.AgentDefinitionStore
	sessionService      session.Service
	llm                 model.LLM
	tools               []tool.Tool
	toolsets            []tool.Toolset
	appName             string
	instructionProvider func(adkagent.ReadonlyContext) (string, error)
}

// NewAgentManager creates a new agent manager.
func NewAgentManager(
	registry *agent.AgentRegistry,
	agentDefStore *mysqlstore.AgentDefinitionStore,
	sessionService session.Service,
	llm model.LLM,
	tools []tool.Tool,
	toolsets []tool.Toolset,
	appName string,
	instructionProvider func(adkagent.ReadonlyContext) (string, error),
) *AgentManager {
	return &AgentManager{
		registry:            registry,
		agentDefStore:       agentDefStore,
		sessionService:      sessionService,
		llm:                 llm,
		tools:               tools,
		toolsets:            toolsets,
		appName:             appName,
		instructionProvider: instructionProvider,
	}
}

// CreateAgent creates a new agent and registers it to the registry.
func (m *AgentManager) CreateAgent(ctx context.Context, def *mysqlstore.AgentDefinition) error {
	// Save to database first
	if err := m.agentDefStore.Create(ctx, def); err != nil {
		return fmt.Errorf("save agent definition: %w", err)
	}

	// Register to runtime registry
	if err := m.registerAgent(def); err != nil {
		slog.Error("failed to register agent to registry", "agent_id", def.ID, "err", err)
		// Don't fail the creation, agent is saved in DB
	}

	return nil
}

// UpdateAgent updates an existing agent and re-registers it.
func (m *AgentManager) UpdateAgent(ctx context.Context, def *mysqlstore.AgentDefinition) error {
	// Update database
	if err := m.agentDefStore.Update(ctx, def); err != nil {
		return fmt.Errorf("update agent definition: %w", err)
	}

	// Re-register to runtime registry
	if err := m.registerAgent(def); err != nil {
		slog.Error("failed to re-register agent to registry", "agent_id", def.ID, "err", err)
	}

	return nil
}

// DeleteAgent removes an agent from database and registry.
func (m *AgentManager) DeleteAgent(ctx context.Context, agentID string) error {
	// Delete from database
	if err := m.agentDefStore.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("delete agent definition: %w", err)
	}

	// TODO: Remove from registry (registry doesn't support removal yet)
	slog.Warn("agent deleted from database but remains in registry until restart", "agent_id", agentID)

	return nil
}

// LoadAllAgents loads all agents from database and registers them.
// If tenantID is empty, loads agents from all tenants.
func (m *AgentManager) LoadAllAgents(ctx context.Context, tenantID string) error {
	var agents []*mysqlstore.AgentDefinition
	var err error

	if tenantID == "" {
		// Load all agents from all tenants
		agents, err = m.agentDefStore.ListAll(ctx)
	} else {
		// Load agents for specific tenant
		agents, err = m.agentDefStore.List(ctx, tenantID)
	}

	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	for _, agentDef := range agents {
		if err := m.registerAgent(agentDef); err != nil {
			slog.Error("failed to register agent", "agent_id", agentDef.ID, "err", err)
			continue
		}
	}

	slog.Info("loaded agents from database", "count", len(agents))
	return nil
}

// RegisterAgent registers an agent to the runtime registry (public method for API).
func (m *AgentManager) RegisterAgent(def *mysqlstore.AgentDefinition) error {
	return m.registerAgent(def)
}

// registerAgent creates an ADK agent and registers it to the registry.
// Uses the shared ContextBuilder InstructionProvider which loads IDENTITY, SOUL, RULES, AGENT
// from workspace docs instead of building from config fields.
func (m *AgentManager) registerAgent(def *mysqlstore.AgentDefinition) error {
	// Create LLM agent using shared InstructionProvider from ContextBuilder
	agentConfig := llmagent.Config{
		Name:                def.Name,
		Description:         def.Description,
		Model:               m.llm,
		Tools:               m.tools,
		Toolsets:            m.toolsets,
		InstructionProvider: m.instructionProvider,
	}

	adkAgent, err := llmagent.New(agentConfig)
	if err != nil {
		return fmt.Errorf("create llmagent: %w", err)
	}

	// Create runner
	runnerCfg := runner.Config{
		AppName:        m.appName,
		Agent:          adkAgent,
		SessionService: m.sessionService,
	}

	r, err := runner.New(runnerCfg)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	// Build routes from enabled channels in config
	routes := m.buildRoutes(def)

	// Register to registry
	m.registry.Register(&agent.AgentEntry{
		ID:     def.ID,
		Agent:  adkAgent,
		Runner: r,
		Config: types.AgentDefinition{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Model:       def.Model,
			Routes:      routes,
		},
	})

	slog.Info("registered agent", "agent_id", def.ID, "name", def.Name, "channels", len(routes))
	return nil
}

// buildRoutes builds agent routes from config.channels
func (m *AgentManager) buildRoutes(def *mysqlstore.AgentDefinition) []types.AgentRoute {
	var routes []types.AgentRoute

	// Default: always enable web channel
	routes = append(routes, types.AgentRoute{
		AgentID: def.ID,
		Channel: "web",
	})

	// Add routes from config.channels
	if def.Config != nil {
		if channelsMap, ok := def.Config["channels"].(map[string]interface{}); ok {
			for channelID, channelData := range channelsMap {
				if channelID == "web" {
					continue // Already added
				}
				
				// Check if channel is enabled
				if channelObj, ok := channelData.(map[string]interface{}); ok {
					if enabled, ok := channelObj["enabled"].(bool); ok && enabled {
						routes = append(routes, types.AgentRoute{
							AgentID: def.ID,
							Channel: channelID,
						})
					}
				}
			}
		}
	}

	return routes
}
