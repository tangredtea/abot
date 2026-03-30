package telegram

// TelegramConfig holds configuration for the Telegram Bot channel.
type TelegramConfig struct {
	Token       string   `yaml:"token"`
	WebhookURL  string   `yaml:"webhook_url,omitempty"` // optional; if empty, uses long polling
	WebhookPort int      `yaml:"webhook_port,omitempty"`
	WebhookPath string   `yaml:"webhook_path,omitempty"`
	AllowFrom   []string `yaml:"allow_from,omitempty"`
	TenantID    string   `yaml:"tenant_id"`
	PollTimeout int      `yaml:"poll_timeout,omitempty"` // getUpdates timeout in seconds, default 30
}

// Update represents a Telegram Bot API Update object (subset).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message represents a Telegram message (subset of fields we care about).
type Message struct {
	MessageID int64       `json:"message_id"`
	From      *User       `json:"from,omitempty"`
	Chat      Chat        `json:"chat"`
	Text      string      `json:"text,omitempty"`
	Photo     []PhotoSize `json:"photo,omitempty"`
	Voice     *Voice      `json:"voice,omitempty"`
	Document  *Document   `json:"document,omitempty"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"` // "private", "group", "supergroup", "channel"
	Title string `json:"title,omitempty"`
}

// PhotoSize represents one size of a photo.
type PhotoSize struct {
	FileID string `json:"file_id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Voice represents a voice message.
type Voice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
}

// Document represents a general file.
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name,omitempty"`
}

// getUpdatesResponse is the Bot API response for getUpdates.
type getUpdatesResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

// sendMessageRequest is the request body for sendMessage.
type sendMessageRequest struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// apiResponse is a generic Bot API response.
type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}
