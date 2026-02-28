package anthropic

import "encoding/json"

// --- Anthropic API request types ---

type MessagesRequest struct {
	Model         string         `json:"model"`
	MaxTokens     int            `json:"max_tokens"`
	Messages      []APIMessage   `json:"messages"`
	System        []ContentBlock `json:"system,omitempty"`
	Tools         []APITool      `json:"tools,omitempty"`
	Stream        bool           `json:"stream"`
	Temperature   *float32       `json:"temperature,omitempty"`
	TopP          *float32       `json:"top_p,omitempty"`
	TopK          *int32         `json:"top_k,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
}

type APIMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
	Source       *imageSource    `json:"source,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type APITool struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	InputSchema  *toolSchema   `json:"input_schema"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type toolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]*toolSchema `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Items      *toolSchema            `json:"items,omitempty"`
	Enum       []string               `json:"enum,omitempty"`
	Desc       string                 `json:"description,omitempty"`
}

// --- Anthropic API response types ---

type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        APIUsage       `json:"usage"`
}

type APIUsage struct {
	InputTokens   int `json:"input_tokens"`
	OutputTokens  int `json:"output_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens,omitempty"`
	CacheRead     int `json:"cache_read_input_tokens,omitempty"`
}

// --- Streaming event types ---

type streamEvent struct {
	Type         string          `json:"type"`
	Message      json.RawMessage `json:"message,omitempty"`
	Index        int             `json:"index,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	Usage        *APIUsage       `json:"usage,omitempty"`
}

type streamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// --- Error types ---

type apiError struct {
	Type  string         `json:"type"`
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// genai role constants
const (
	roleUser  = "user"
	roleModel = "model"
)
