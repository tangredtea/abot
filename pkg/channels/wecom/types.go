package wecom

import "encoding/xml"

// WeComConfig holds configuration for the WeCom Bot channel.
type WeComConfig struct {
	Token          string   `yaml:"token"`
	EncodingAESKey string   `yaml:"encoding_aes_key"`
	WebhookURL     string   `yaml:"webhook_url"`
	WebhookHost    string   `yaml:"webhook_host"`
	WebhookPort    int      `yaml:"webhook_port"`
	WebhookPath    string   `yaml:"webhook_path"`
	ReplyTimeout   int      `yaml:"reply_timeout"`
	AllowFrom      []string `yaml:"allow_from"`
	TenantID       string   `yaml:"tenant_id"`
	UserID         string   `yaml:"user_id"`
}

// --- XML message structures (default callback format) ---

// XMLBotMessage is the decrypted XML message from WeCom Bot callback.
type XMLBotMessage struct {
	XMLName        xml.Name `xml:"xml"`
	WebhookUrl     string   `xml:"WebhookUrl"`
	MsgId          string   `xml:"MsgId"`
	ChatId         string   `xml:"ChatId"`
	PostId         string   `xml:"PostId"`
	ChatType       string   `xml:"ChatType"`
	From           XMLFrom  `xml:"From"`
	GetChatInfoUrl string   `xml:"GetChatInfoUrl"`
	MsgType        string   `xml:"MsgType"`
	Text           struct {
		Content string `xml:"Content"`
	} `xml:"Text"`
	Image struct {
		ImageUrl string `xml:"ImageUrl"`
	} `xml:"Image"`
	MixedMessage struct {
		MsgItem []XMLMixedItem `xml:"MsgItem"`
	} `xml:"MixedMessage"`
	Event struct {
		EventType string `xml:"EventType"`
	} `xml:"Event"`
	Attachment struct {
		CallbackId string `xml:"CallbackId"`
		Actions    struct {
			Name  string `xml:"Name"`
			Value string `xml:"Value"`
			Type  string `xml:"Type"`
		} `xml:"Actions"`
	} `xml:"Attachment"`
}

type XMLFrom struct {
	UserId string `xml:"UserId"`
	Name   string `xml:"Name"`
	Alias  string `xml:"Alias"`
}

type XMLMixedItem struct {
	MsgType string `xml:"MsgType"`
	Text    struct {
		Content string `xml:"Content"`
	} `xml:"Text"`
	Image struct {
		ImageUrl string `xml:"ImageUrl"`
	} `xml:"Image"`
}

// --- JSON message structures (robot_callback_format=json) ---

// JSONBotMessage is the decrypted JSON message from WeCom Bot callback.
type JSONBotMessage struct {
	WebhookURL     string   `json:"webhook_url"`
	MsgID          string   `json:"msgid"`
	ChatID         string   `json:"chatid"`
	PostID         string   `json:"postid"`
	ChatType       string   `json:"chattype"`
	From           JSONFrom `json:"from"`
	GetChatInfoURL string   `json:"get_chat_info_url"`
	MsgType        string   `json:"msgtype"`
	Text           struct {
		Content string `json:"content"`
	} `json:"text"`
	Image struct {
		ImageURL string `json:"image_url"`
	} `json:"image"`
	MixedMessage struct {
		MsgItem []JSONMixedItem `json:"msg_item"`
	} `json:"mixed_message"`
	Event struct {
		EventType string `json:"event_type"`
	} `json:"event"`
	Attachment struct {
		CallbackID string `json:"callback_id"`
		Actions    []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"actions"`
	} `json:"attachment"`
}

type JSONFrom struct {
	UserID string `json:"userid"`
	Name   string `json:"name"`
	Alias  string `json:"alias"`
}

type JSONMixedItem struct {
	MsgType string `json:"msg_type"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Image struct {
		ImageURL string `json:"image_url"`
	} `json:"image"`
}

// BotMessage is the unified internal representation after parsing XML or JSON.
type BotMessage struct {
	WebhookURL     string
	MsgID          string
	ChatID         string
	PostID         string
	ChatType       string
	FromUserID     string
	FromName       string
	FromAlias      string
	GetChatInfoURL string
	MsgType        string
	TextContent    string
	ImageURL       string
	MixedItems     []MixedItem
	EventType      string
	CallbackID     string
}

type MixedItem struct {
	MsgType  string
	Content  string
	ImageURL string
}

// ReplyMessage represents the reply message structure for WeCom Bot webhook.
type ReplyMessage struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text,omitempty"`
}
