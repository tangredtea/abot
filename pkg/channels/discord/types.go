package discord

import "encoding/json"

// DiscordConfig holds configuration for the Discord Bot channel.
type DiscordConfig struct {
	Token     string   `yaml:"token"`
	AllowFrom []string `yaml:"allow_from,omitempty"`
	TenantID  string   `yaml:"tenant_id"`
	GuildID   string   `yaml:"guild_id,omitempty"` // optional filter
}

// Gateway payload types (subset).

type gatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int64          `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type identifyPayload struct {
	Token      string            `json:"token"`
	Intents    int               `json:"intents"`
	Properties map[string]string `json:"properties"`
}

type helloPayload struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type readyPayload struct {
	SessionID string `json:"session_id"`
}

// Discord message types (subset).

type discordMessage struct {
	ID        string       `json:"id"`
	ChannelID string       `json:"channel_id"`
	GuildID   string       `json:"guild_id,omitempty"`
	Author    discordUser  `json:"author"`
	Content   string       `json:"content"`
}

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// REST API types.

type sendMessageBody struct {
	Content string `json:"content"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
