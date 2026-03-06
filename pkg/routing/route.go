package routing

import (
	"strings"

	"abot/pkg/types"
)

// RouteInput contains the routing context for an inbound message.
type RouteInput struct {
	Channel    string
	AccountID  string
	Peer       *RoutePeer
	ParentPeer *RoutePeer
	GuildID    string
	TeamID     string
}

// ResolvedRoute is the result of route resolution.
type ResolvedRoute struct {
	AgentID        string
	Channel        string
	AccountID      string
	SessionKey     string
	MainSessionKey string
	MatchedBy      string // "binding.peer", "binding.peer.parent", "binding.guild", "binding.team", "binding.account", "binding.channel", "default"
}

// RouteConfig is the configuration input for the route resolver.
type RouteConfig struct {
	Agents        []types.AgentDefinition
	DMScope       DMScope
	IdentityLinks map[string][]string
}

// RouteResolver determines which agent handles a message based on configured bindings.
type RouteResolver struct {
	cfg RouteConfig
}

// NewRouteResolver creates a route resolver.
func NewRouteResolver(cfg RouteConfig) *RouteResolver {
	return &RouteResolver{cfg: cfg}
}

// ResolveRoute determines which agent handles a message and builds the session key.
// Implements a 7-level priority cascade:
// peer > parent_peer > guild > team > account > channel_wildcard > default
func (r *RouteResolver) ResolveRoute(input RouteInput) ResolvedRoute {
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	accountID := NormalizeAccountID(input.AccountID)
	peer := input.Peer

	dmScope := r.cfg.DMScope
	if dmScope == "" {
		dmScope = DMScopeMain
	}

	routes := r.collectRoutes(channel, accountID)

	choose := func(agentID, matchedBy string) ResolvedRoute {
		resolved := r.pickAgentID(agentID)
		sk := strings.ToLower(BuildAgentPeerSessionKey(SessionKeyParams{
			AgentID:       resolved,
			Channel:       channel,
			AccountID:     accountID,
			Peer:          peer,
			DMScope:       dmScope,
			IdentityLinks: r.cfg.IdentityLinks,
		}))
		msk := strings.ToLower(BuildAgentMainSessionKey(resolved))
		return ResolvedRoute{
			AgentID:        resolved,
			Channel:        channel,
			AccountID:      accountID,
			SessionKey:     sk,
			MainSessionKey: msk,
			MatchedBy:      matchedBy,
		}
	}

	// Priority 1: peer binding.
	if peer != nil && strings.TrimSpace(peer.ID) != "" {
		if m := findPeerMatch(routes, peer); m != nil {
			return choose(m.AgentID, "binding.peer")
		}
	}

	// Priority 2: parent peer binding.
	if pp := input.ParentPeer; pp != nil && strings.TrimSpace(pp.ID) != "" {
		if m := findPeerMatch(routes, pp); m != nil {
			return choose(m.AgentID, "binding.peer.parent")
		}
	}

	// Priority 3: guild binding.
	if gid := strings.TrimSpace(input.GuildID); gid != "" {
		if m := findGuildMatch(routes, gid); m != nil {
			return choose(m.AgentID, "binding.guild")
		}
	}

	// Priority 4: team binding.
	if tid := strings.TrimSpace(input.TeamID); tid != "" {
		if m := findTeamMatch(routes, tid); m != nil {
			return choose(m.AgentID, "binding.team")
		}
	}

	// Priority 5: account binding.
	if m := findAccountMatch(routes); m != nil {
		return choose(m.AgentID, "binding.account")
	}

	// Priority 6: channel wildcard binding.
	if m := findChannelWildcard(routes); m != nil {
		return choose(m.AgentID, "binding.channel")
	}

	// Priority 7: default agent.
	return choose(r.resolveDefaultAgentID(), "default")
}

// collectRoutes gathers routes matching the channel and accountID from all agent definitions.
func (r *RouteResolver) collectRoutes(channel, accountID string) []types.AgentRoute {
	var filtered []types.AgentRoute
	for _, agent := range r.cfg.Agents {
		for _, route := range agent.Routes {
			route.AgentID = agent.ID
			matchCh := strings.ToLower(strings.TrimSpace(route.Channel))
			if matchCh == "" || matchCh != channel {
				continue
			}
			if !matchesAccountID(route.AccountID, accountID) {
				continue
			}
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func matchesAccountID(matchAccountID, actual string) bool {
	trimmed := strings.TrimSpace(matchAccountID)
	if trimmed == "" {
		return actual == DefaultAccountID
	}
	if trimmed == "*" {
		return true
	}
	return strings.ToLower(trimmed) == strings.ToLower(actual)
}

func findPeerMatch(routes []types.AgentRoute, peer *RoutePeer) *types.AgentRoute {
	for i := range routes {
		r := &routes[i]
		if r.Peer == nil {
			continue
		}
		peerKind := strings.ToLower(strings.TrimSpace(r.Peer.Kind))
		peerID := strings.TrimSpace(r.Peer.ID)
		if peerKind == "" || peerID == "" {
			continue
		}
		if peerKind == strings.ToLower(peer.Kind) && peerID == peer.ID {
			return r
		}
	}
	return nil
}

func findGuildMatch(routes []types.AgentRoute, guildID string) *types.AgentRoute {
	for i := range routes {
		r := &routes[i]
		if mg := strings.TrimSpace(r.GuildID); mg != "" && mg == guildID {
			return r
		}
	}
	return nil
}

func findTeamMatch(routes []types.AgentRoute, teamID string) *types.AgentRoute {
	for i := range routes {
		r := &routes[i]
		if mt := strings.TrimSpace(r.TeamID); mt != "" && mt == teamID {
			return r
		}
	}
	return nil
}

func findAccountMatch(routes []types.AgentRoute) *types.AgentRoute {
	for i := range routes {
		r := &routes[i]
		aid := strings.TrimSpace(r.AccountID)
		if aid == "*" {
			continue
		}
		if r.Peer != nil || r.GuildID != "" || r.TeamID != "" {
			continue
		}
		return &routes[i]
	}
	return nil
}

func findChannelWildcard(routes []types.AgentRoute) *types.AgentRoute {
	for i := range routes {
		r := &routes[i]
		aid := strings.TrimSpace(r.AccountID)
		if aid != "*" {
			continue
		}
		if r.Peer != nil || r.GuildID != "" || r.TeamID != "" {
			continue
		}
		return &routes[i]
	}
	return nil
}

// pickAgentID validates that agentID exists in the config; falls back to default if not.
func (r *RouteResolver) pickAgentID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return NormalizeAgentID(r.resolveDefaultAgentID())
	}
	normalized := NormalizeAgentID(trimmed)
	agents := r.cfg.Agents
	if len(agents) == 0 {
		return normalized
	}
	for _, a := range agents {
		if NormalizeAgentID(a.ID) == normalized {
			return normalized
		}
	}
	return NormalizeAgentID(r.resolveDefaultAgentID())
}

// resolveDefaultAgentID finds the agent marked as default, or returns the first one.
func (r *RouteResolver) resolveDefaultAgentID() string {
	agents := r.cfg.Agents
	if len(agents) == 0 {
		return DefaultAgentID
	}
	for _, a := range agents {
		if a.IsDefault {
			if id := strings.TrimSpace(a.ID); id != "" {
				return NormalizeAgentID(id)
			}
		}
	}
	if id := strings.TrimSpace(agents[0].ID); id != "" {
		return NormalizeAgentID(id)
	}
	return DefaultAgentID
}
