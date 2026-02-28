package routing

import (
	"fmt"
	"strings"
)

// DMScope controls the isolation granularity of DM sessions.
type DMScope string

const (
	DMScopeMain                  DMScope = "main"
	DMScopePerPeer               DMScope = "per-peer"
	DMScopePerChannelPeer        DMScope = "per-channel-peer"
	DMScopePerAccountChannelPeer DMScope = "per-account-channel-peer"
)

// RoutePeer represents a chat peer with a kind and ID.
type RoutePeer struct {
	Kind string // "direct", "group", "channel"
	ID   string
}

// SessionKeyParams holds all inputs needed to build a session key.
type SessionKeyParams struct {
	AgentID       string
	Channel       string
	AccountID     string
	Peer          *RoutePeer
	DMScope       DMScope
	IdentityLinks map[string][]string // cross-platform identity merge mapping
}

// ParsedSessionKey is the result of parsing an agent session key.
type ParsedSessionKey struct {
	AgentID string
	Rest    string
}

// BuildAgentMainSessionKey returns "agent:<agentId>:main".
func BuildAgentMainSessionKey(agentID string) string {
	return fmt.Sprintf("agent:%s:%s", NormalizeAgentID(agentID), DefaultMainKey)
}

// BuildAgentPeerSessionKey builds a session key from agent, channel, peer, and DM scope.
// Direct peers use DMScope to determine isolation granularity; group/channel peers
// are always isolated per peer.
func BuildAgentPeerSessionKey(params SessionKeyParams) string {
	agentID := NormalizeAgentID(params.AgentID)

	peer := params.Peer
	if peer == nil {
		peer = &RoutePeer{Kind: "direct"}
	}
	peerKind := strings.TrimSpace(peer.Kind)
	if peerKind == "" {
		peerKind = "direct"
	}

	if peerKind == "direct" {
		return buildDirectSessionKey(agentID, params, peer)
	}

	// group/channel peers are always isolated per peer.
	channel := normalizeChannel(params.Channel)
	peerID := strings.ToLower(strings.TrimSpace(peer.ID))
	if peerID == "" {
		peerID = "unknown"
	}
	return fmt.Sprintf("agent:%s:%s:%s:%s", agentID, channel, peerKind, peerID)
}

// buildDirectSessionKey handles session key construction for direct-type peers.
func buildDirectSessionKey(agentID string, params SessionKeyParams, peer *RoutePeer) string {
	dmScope := params.DMScope
	if dmScope == "" {
		dmScope = DMScopeMain
	}
	peerID := strings.TrimSpace(peer.ID)

	// Cross-platform identity merge: map peerID to a unified identity if configured.
	if dmScope != DMScopeMain && peerID != "" {
		if linked := resolveLinkedPeerID(params.IdentityLinks, params.Channel, peerID); linked != "" {
			peerID = linked
		}
	}
	peerID = strings.ToLower(peerID)

	switch dmScope {
	case DMScopePerAccountChannelPeer:
		if peerID != "" {
			channel := normalizeChannel(params.Channel)
			accountID := NormalizeAccountID(params.AccountID)
			return fmt.Sprintf("agent:%s:%s:%s:direct:%s", agentID, channel, accountID, peerID)
		}
	case DMScopePerChannelPeer:
		if peerID != "" {
			channel := normalizeChannel(params.Channel)
			return fmt.Sprintf("agent:%s:%s:direct:%s", agentID, channel, peerID)
		}
	case DMScopePerPeer:
		if peerID != "" {
			return fmt.Sprintf("agent:%s:direct:%s", agentID, peerID)
		}
	}
	return BuildAgentMainSessionKey(agentID)
}

// ParseAgentSessionKey extracts agentId and rest from "agent:<agentId>:<rest>".
func ParseAgentSessionKey(sessionKey string) *ParsedSessionKey {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return nil
	}
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) < 3 || parts[0] != "agent" {
		return nil
	}
	agentID := strings.TrimSpace(parts[1])
	rest := parts[2]
	if agentID == "" || rest == "" {
		return nil
	}
	return &ParsedSessionKey{AgentID: agentID, Rest: rest}
}

// IsSubagentSessionKey reports whether the session key belongs to a sub-agent.
func IsSubagentSessionKey(sessionKey string) bool {
	raw := strings.TrimSpace(sessionKey)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(raw), "subagent:") {
		return true
	}
	parsed := ParseAgentSessionKey(raw)
	if parsed == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(parsed.Rest), "subagent:")
}

func normalizeChannel(channel string) string {
	c := strings.TrimSpace(strings.ToLower(channel))
	if c == "" {
		return "unknown"
	}
	return c
}

// resolveLinkedPeerID maps a channel:peerID to a unified identity name via identity links.
func resolveLinkedPeerID(identityLinks map[string][]string, channel, peerID string) string {
	if len(identityLinks) == 0 {
		return ""
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return ""
	}

	candidates := make(map[string]bool)
	candidates[strings.ToLower(peerID)] = true
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "" {
		candidates[fmt.Sprintf("%s:%s", channel, strings.ToLower(peerID))] = true
	}

	for canonical, ids := range identityLinks {
		canonicalName := strings.TrimSpace(canonical)
		if canonicalName == "" {
			continue
		}
		for _, id := range ids {
			normalized := strings.ToLower(strings.TrimSpace(id))
			if normalized != "" && candidates[normalized] {
				return canonicalName
			}
		}
	}
	return ""
}
