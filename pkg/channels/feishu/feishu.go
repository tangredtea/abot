package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"abot/pkg/channels"
	"abot/pkg/types"
)

const ChannelName = "feishu"

// FeishuChannel implements types.Channel for Feishu Bot.
type FeishuChannel struct {
	*channels.BaseChannel
	config      FeishuConfig
	tenantID    string
	accessToken string
	tokenExpiry time.Time
	tokenMu     sync.Mutex
	server      *http.Server
	cancel      context.CancelFunc
}

// NewFeishuChannel creates a new Feishu Bot channel.
func NewFeishuChannel(cfg FeishuConfig, bus types.MessageBus) (*FeishuChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("feishu: app_id and app_secret are required")
	}
	if cfg.VerificationToken == "" {
		return nil, fmt.Errorf("feishu: verification_token is required")
	}
	tid := cfg.TenantID
	if tid == "" {
		tid = types.DefaultTenantID
	}
	return &FeishuChannel{
		BaseChannel: channels.NewBaseChannel(ChannelName, bus, cfg.AllowFrom),
		config:      cfg,
		tenantID:    tid,
	}, nil
}

// Start launches the HTTP webhook server for Feishu event subscription.
func (c *FeishuChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}

	_, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	mux := http.NewServeMux()
	path := c.config.WebhookPath
	if path == "" {
		path = "/webhook/feishu"
	}
	mux.HandleFunc(path, c.handleWebhook)

	addr := fmt.Sprintf("%s:%d", c.config.WebhookHost, c.config.WebhookPort)
	c.server = &http.Server{Addr: addr, Handler: mux}

	c.SetRunning(true)
	slog.Info("feishu: channel started", "addr", addr, "path", path)

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("feishu: http server error", "err", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (c *FeishuChannel) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.server != nil {
		shutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = c.server.Shutdown(shutCtx)
	}
	c.SetRunning(false)
	slog.Info("feishu: channel stopped")
	return nil
}

// Send delivers an outbound message via the Feishu REST API.
func (c *FeishuChannel) Send(ctx context.Context, msg types.OutboundMessage) error {
	if msg.Content == "" {
		return nil
	}
	return c.replyMessage(ctx, msg.ChatID, msg.Content)
}

// handleWebhook routes Feishu event subscription callbacks.
func (c *FeishuChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var envelope struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
		Header    *struct {
			EventType string `json:"event_type"`
			Token     string `json:"token"`
		} `json:"header"`
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// URL verification challenge.
	if envelope.Challenge != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": envelope.Challenge})
		return
	}

	// Verify token.
	token := envelope.Token
	if envelope.Header != nil {
		token = envelope.Header.Token
	}
	if token != c.config.VerificationToken {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}

	// Route event.
	eventType := ""
	if envelope.Header != nil {
		eventType = envelope.Header.EventType
	}
	if eventType == "im.message.receive_v1" {
		go c.processEvent(r.Context(), envelope.Event)
	}

	w.WriteHeader(http.StatusOK)
}

// processEvent extracts message content from a Feishu im.message.receive_v1 event.
func (c *FeishuChannel) processEvent(ctx context.Context, raw json.RawMessage) {
	var event struct {
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"` // "p2p" or "group"
			MessageType string `json:"message_type"`
			Content     string `json:"content"` // JSON string
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		slog.Warn("feishu: parse event", "err", err)
		return
	}

	senderID := event.Sender.SenderID.OpenID
	chatID := event.Message.ChatID
	if chatID == "" {
		chatID = senderID
	}

	// Parse content JSON — Feishu wraps text in {"text":"..."}
	var content string
	if event.Message.MessageType == "text" {
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(event.Message.Content), &textContent); err == nil {
			content = textContent.Text
		}
	}

	if content == "" {
		return
	}

	metadata := map[string]string{
		"platform":   "feishu",
		"message_id": event.Message.MessageID,
		"chat_type":  event.Message.ChatType,
	}

	_ = c.HandleMessage(ctx, c.tenantID, senderID, senderID, chatID, content, nil, metadata)
}

// getAccessToken returns a valid tenant_access_token, refreshing if expired.
func (c *FeishuChannel) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	payload, _ := json.Marshal(map[string]string{
		"app_id":     c.config.AppID,
		"app_secret": c.config.AppSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("feishu: parse token: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: token error %d: %s", result.Code, result.Msg)
	}

	c.accessToken = result.TenantAccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.Expire-60) * time.Second)
	return c.accessToken, nil
}

// replyMessage sends a text message to a Feishu chat via REST API.
func (c *FeishuChannel) replyMessage(ctx context.Context, chatID, text string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return err
	}

	contentJSON, _ := json.Marshal(map[string]string{"text": text})
	payload, _ := json.Marshal(map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(contentJSON),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("feishu: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu: API error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
