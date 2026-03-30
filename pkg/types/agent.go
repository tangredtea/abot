package types

// PeerMatch describes a peer-level routing match condition.
type PeerMatch struct {
	Kind string // "direct", "group", or "channel"
	ID   string
}

// AgentRoute maps inbound messages to an agent, supporting multi-level matching:
// channel, chatID, peer, guild, team, and account.
type AgentRoute struct {
	AgentID   string
	Channel   string
	ChatID    string
	AccountID string // Account ID ("*" for wildcard).
	Peer      *PeerMatch
	GuildID   string // Discord guild, etc.
	TeamID    string // Slack team, etc.
	Priority  int
	IsDefault bool // Marks this as the default agent.
}

// AgentDefinition holds the configuration for a single agent instance.
type AgentDefinition struct {
	ID          string
	Name        string
	Description string
	Model       string
	IsDefault   bool // Marks this as the default agent.
	Routes      []AgentRoute
}

// TaskSummary is a brief summary of a sub-task, used by the SubagentSpawner interface.
type TaskSummary struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id,omitempty"`
	Status  string `json:"status"`
	Task    string `json:"task"`
}
