package agent

import (
	"sync"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"

	"abot/pkg/types"
)

// AgentEntry holds an agent, its runner, and definition.
type AgentEntry struct {
	ID     string
	Agent  adkagent.Agent
	Runner *runner.Runner
	Config types.AgentDefinition
}

// AgentRegistry manages multiple agents and routes inbound messages.
type AgentRegistry struct {
	agents map[string]*AgentEntry
	routes []types.AgentRoute
	mu     sync.RWMutex
}

// NewAgentRegistry creates an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]*AgentEntry),
	}
}

// Register adds an agent entry and its routes.
func (r *AgentRegistry) Register(entry *AgentEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[entry.ID] = entry
	for _, route := range entry.Config.Routes {
		route.AgentID = entry.ID
		r.routes = append(r.routes, route)
	}
}

// GetRunner returns the runner for the given agent ID.
func (r *AgentRegistry) GetRunner(agentID string) (*runner.Runner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.agents[agentID]
	if !ok {
		return nil, false
	}
	return e.Runner, true
}

// ListAgents returns all registered agent IDs.
func (r *AgentRegistry) ListAgents() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}

// ResolveRoute finds the best matching agent for an inbound message.
// Matching priority: exact channel+chatID > channel-only > default (first registered).
func (r *AgentRegistry) ResolveRoute(msg types.InboundMessage) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var bestRoute *types.AgentRoute
	bestScore := -1

	for i := range r.routes {
		route := &r.routes[i]
		score := MatchScore(route, msg)
		if score > bestScore {
			bestScore = score
			bestRoute = route
		}
	}

	if bestRoute != nil {
		return bestRoute.AgentID
	}

	// Fallback: return first registered agent.
	for id := range r.agents {
		return id
	}
	return ""
}

// GetEntry returns the full agent entry (including runner, config, etc.) for the given ID.
func (r *AgentRegistry) GetEntry(agentID string) (*AgentEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.agents[agentID]
	return e, ok
}

// GetDefaultAgent returns the first registered agent as the default.
// Used for fallback routing and startup information.
func (r *AgentRegistry) GetDefaultAgent() *AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.agents {
		return e
	}
	return nil
}

// MatchScore returns how well a route matches a message.
// -1 = no match, 0 = wildcard, 1 = channel match, 2 = channel+chatID match.
func MatchScore(route *types.AgentRoute, msg types.InboundMessage) int {
	if route.Channel != "" && route.Channel != msg.Channel {
		return -1
	}
	score := 0
	if route.Channel == msg.Channel && route.Channel != "" {
		score = 1
	}
	if route.ChatID != "" {
		if route.ChatID != msg.ChatID {
			return -1
		}
		score = 2
	}
	return score
}
