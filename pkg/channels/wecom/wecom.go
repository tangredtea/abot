package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"abot/pkg/channels"
	"abot/pkg/types"
	"golang.org/x/sync/semaphore"
)

const ChannelName = "wecom"

const (
	// dedupTTL is how long message IDs are remembered for deduplication.
	dedupTTL = 5 * time.Minute
	// shutdownTimeout is the grace period for HTTP server shutdown.
	shutdownTimeout = 5 * time.Second
)

// dedupCache is a TTL-based message deduplication cache with background cleanup.
type dedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
	maxSize int
	stop    chan struct{}
}

const defaultDedupMaxSize = 100000

func NewDedupCache(ttl time.Duration) *dedupCache {
	d := &dedupCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		maxSize: defaultDedupMaxSize,
		stop:    make(chan struct{}),
	}
	go d.backgroundCleanup()
	return d
}

// Check returns true if id was already seen (duplicate).
func (d *dedupCache) Check(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.entries[id]; ok {
		return true
	}
	// Evict oldest if at capacity.
	if len(d.entries) >= d.maxSize {
		d.evictOldest()
	}
	d.entries[id] = time.Now()
	return false
}

// Close stops the background cleanup goroutine.
func (d *dedupCache) Close() {
	select {
	case <-d.stop:
	default:
		close(d.stop)
	}
}

func (d *dedupCache) backgroundCleanup() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.mu.Lock()
			d.cleanup()
			d.mu.Unlock()
		}
	}
}

func (d *dedupCache) cleanup() {
	cutoff := time.Now().Add(-d.ttl)
	for k, t := range d.entries {
		if t.Before(cutoff) {
			delete(d.entries, k)
		}
	}
}

func (d *dedupCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, t := range d.entries {
		if oldestKey == "" || t.Before(oldestTime) {
			oldestKey = k
			oldestTime = t
		}
	}
	if oldestKey != "" {
		delete(d.entries, oldestKey)
	}
}

// WeComChannel implements types.Channel for WeCom Bot (企业微信智能机器人).
type WeComChannel struct {
	*channels.BaseChannel
	config   WeComConfig
	server   *http.Server
	dedup    *dedupCache
	tenantID string
	userID   string
	ctx      context.Context
	cancel   context.CancelFunc
	sem      *semaphore.Weighted
}

// NewWeComChannel creates a new WeCom Bot channel.
func NewWeComChannel(cfg WeComConfig, bus types.MessageBus) (*WeComChannel, error) {
	if cfg.Token == "" || cfg.WebhookURL == "" {
		return nil, fmt.Errorf("wecom: token and webhook_url are required")
	}
	return &WeComChannel{
		BaseChannel: channels.NewBaseChannel(ChannelName, bus, cfg.AllowFrom),
		config:      cfg,
		dedup:       NewDedupCache(dedupTTL),
		tenantID:    cfg.TenantID,
		userID:      cfg.UserID,
		sem:         semaphore.NewWeighted(100), // Limit concurrent message processing
	}, nil
}

// Start launches the HTTP webhook server.
func (c *WeComChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	mux := http.NewServeMux()
	path := c.config.WebhookPath
	if path == "" {
		path = "/webhook/wecom"
	}
	mux.HandleFunc(path, c.HandleWebhook)
	// Also handle doubled path (e.g. /webhook/wecom/webhook/wecom) caused by
	// reverse proxies like SGW that prepend the backend path to the original path.
	doubled := path + path
	mux.HandleFunc(doubled, c.HandleWebhook)
	// Catch-all: log any request that doesn't match the webhook path.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Warn("wecom: catch-all hit",
			"method", r.Method,
			"url", r.URL.String(),
			"request_uri", r.RequestURI,
			"remote", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf("%s:%d", c.config.WebhookHost, c.config.WebhookPort)
	c.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	c.SetRunning(true)
	slog.Info("wecom: channel started", "addr", addr, "path", path)

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("wecom: http server error", "err", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (c *WeComChannel) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.dedup != nil {
		c.dedup.Close()
	}
	if c.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()
		_ = c.server.Shutdown(shutdownCtx)
	}
	c.SetRunning(false)
	slog.Info("wecom: channel stopped")
	return nil
}

// Send delivers an outbound message via the response_url stored in metadata,
// falling back to the configured WebhookURL.
func (c *WeComChannel) Send(ctx context.Context, msg types.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("wecom: channel not running")
	}
	url := c.config.WebhookURL
	if u, ok := msg.Metadata["response_url"]; ok && u != "" {
		url = u
	}
	return c.sendReply(ctx, url, msg.Content)
}

// HandleWebhook routes GET (verification) and POST (message callback).
func (c *WeComChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Dump all headers for debugging proxy behavior.
	var hdrs []string
	for k, v := range r.Header {
		hdrs = append(hdrs, fmt.Sprintf("%s=%s", k, strings.Join(v, ",")))
	}
	slog.Info("wecom: incoming request",
		"method", r.Method,
		"url", r.URL.String(),
		"raw_query", r.URL.RawQuery,
		"request_uri", r.RequestURI,
		"remote", r.RemoteAddr,
		"headers", strings.Join(hdrs, "; "))

	switch r.Method {
	case http.MethodGet:
		c.HandleVerification(w, r)
	case http.MethodPost:
		c.HandleMessageCallback(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleVerification handles the URL verification GET request from WeCom.
func (c *WeComChannel) HandleVerification(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSig := q.Get("msg_signature")
	ts := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	slog.Info("wecom: verification request",
		"msg_signature", msgSig, "timestamp", ts, "nonce", nonce,
		"echostr_len", len(echostr))

	if msgSig == "" || ts == "" || nonce == "" || echostr == "" {
		// Return 200 for bare GET (proxy health check / connectivity test).
		slog.Warn("wecom: verification missing params, returning 200 for health check")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	if !VerifySignature(c.config.Token, msgSig, ts, nonce, echostr) {
		slog.Warn("wecom: signature verification failed", "msg_signature", msgSig)
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	slog.Info("wecom: signature verified OK")

	// For AIBOT, receiveid is empty string.
	decrypted, err := DecryptMessage(echostr, c.config.EncodingAESKey, "")
	if err != nil {
		slog.Error("wecom: decrypt echostr failed", "err", err)
		http.Error(w, "Decryption failed", http.StatusInternalServerError)
		return
	}

	decrypted = strings.TrimSpace(decrypted)
	decrypted = strings.TrimPrefix(decrypted, "\xef\xbb\xbf")
	slog.Info("wecom: verification success", "echostr_decrypted_len", len(decrypted))
	w.Write([]byte(decrypted))
}

// HandleMessageCallback handles incoming POST message callbacks from WeCom.
func (c *WeComChannel) HandleMessageCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSig := q.Get("msg_signature")
	ts := q.Get("timestamp")
	nonce := q.Get("nonce")

	slog.Info("wecom: message callback", "msg_signature", msgSig, "timestamp", ts, "nonce", nonce)

	if msgSig == "" || ts == "" || nonce == "" {
		slog.Warn("wecom: callback missing params")
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse encrypted envelope (always XML per WeCom protocol).
	var envelope struct {
		XMLName xml.Name `xml:"xml"`
		Encrypt string   `xml:"Encrypt"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		slog.Error("wecom: parse envelope failed", "err", err)
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}

	if !VerifySignature(c.config.Token, msgSig, ts, nonce, envelope.Encrypt) {
		slog.Warn("wecom: callback signature failed")
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	decrypted, err := DecryptMessage(envelope.Encrypt, c.config.EncodingAESKey, "")
	if err != nil {
		slog.Error("wecom: decrypt message failed", "err", err)
		http.Error(w, "Decryption failed", http.StatusInternalServerError)
		return
	}

	slog.Info("wecom: decrypted message", "len", len(decrypted), "preview", truncateStr(decrypted, 200))

	msg, err := parseDecryptedMessage(decrypted)
	if err != nil {
		slog.Error("wecom: parse decrypted message failed", "err", err)
		http.Error(w, "Invalid message format", http.StatusBadRequest)
		return
	}

	go func() {
		if err := c.sem.Acquire(c.ctx, 1); err != nil {
			slog.Warn("wecom: failed to acquire semaphore", "err", err)
			return
		}
		defer c.sem.Release(1)
		c.processMessage(c.ctx, msg)
	}()

	// Return 200 with empty body; reply asynchronously via webhook URL.
	w.WriteHeader(http.StatusOK)
}

// truncateStr returns the first n bytes of s for logging.
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseDecryptedMessage tries JSON first, then XML, and returns a unified BotMessage.
func parseDecryptedMessage(data string) (BotMessage, error) {
	trimmed := strings.TrimSpace(data)

	// Try JSON first (robot_callback_format=json).
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return parseJSONMessage(trimmed)
	}
	// Default: XML format.
	return parseXMLMessage(trimmed)
}

func parseJSONMessage(data string) (BotMessage, error) {
	var jm JSONBotMessage
	if err := json.Unmarshal([]byte(data), &jm); err != nil {
		return BotMessage{}, fmt.Errorf("json: %w", err)
	}
	msg := BotMessage{
		WebhookURL:     jm.WebhookURL,
		MsgID:          jm.MsgID,
		ChatID:         jm.ChatID,
		PostID:         jm.PostID,
		ChatType:       jm.ChatType,
		FromUserID:     jm.From.UserID,
		FromName:       jm.From.Name,
		FromAlias:      jm.From.Alias,
		GetChatInfoURL: jm.GetChatInfoURL,
		MsgType:        jm.MsgType,
		TextContent:    jm.Text.Content,
		ImageURL:       jm.Image.ImageURL,
		EventType:      jm.Event.EventType,
		CallbackID:     jm.Attachment.CallbackID,
	}
	for _, item := range jm.MixedMessage.MsgItem {
		msg.MixedItems = append(msg.MixedItems, MixedItem{
			MsgType:  item.MsgType,
			Content:  item.Text.Content,
			ImageURL: item.Image.ImageURL,
		})
	}
	return msg, nil
}

func parseXMLMessage(data string) (BotMessage, error) {
	var xm XMLBotMessage
	if err := xml.Unmarshal([]byte(data), &xm); err != nil {
		return BotMessage{}, fmt.Errorf("xml: %w", err)
	}
	msg := BotMessage{
		WebhookURL:     xm.WebhookUrl,
		MsgID:          xm.MsgId,
		ChatID:         xm.ChatId,
		PostID:         xm.PostId,
		ChatType:       xm.ChatType,
		FromUserID:     xm.From.UserId,
		FromName:       xm.From.Name,
		FromAlias:      xm.From.Alias,
		GetChatInfoURL: xm.GetChatInfoUrl,
		MsgType:        xm.MsgType,
		TextContent:    xm.Text.Content,
		ImageURL:       xm.Image.ImageUrl,
		EventType:      xm.Event.EventType,
		CallbackID:     xm.Attachment.CallbackId,
	}
	for _, item := range xm.MixedMessage.MsgItem {
		msg.MixedItems = append(msg.MixedItems, MixedItem{
			MsgType:  item.MsgType,
			Content:  item.Text.Content,
			ImageURL: item.Image.ImageUrl,
		})
	}
	return msg, nil
}

// processMessage extracts content from the bot message and publishes it to the bus.
func (c *WeComChannel) processMessage(ctx context.Context, msg BotMessage) {
	switch msg.MsgType {
	case "text", "image", "mixed":
	case "event":
		slog.Info("wecom: event received", "event_type", msg.EventType, "chat_id", msg.ChatID)
		return
	default:
		slog.Debug("wecom: skipping unsupported msg type", "type", msg.MsgType)
		return
	}

	if c.dedup.Check(msg.MsgID) {
		slog.Debug("wecom: skipping duplicate", "msg_id", msg.MsgID)
		return
	}

	senderID := msg.FromUserID
	isGroup := msg.ChatType == "group"

	chatID := senderID
	if isGroup {
		chatID = msg.ChatID
	}

	var content string
	var media []string
	switch msg.MsgType {
	case "text":
		content = msg.TextContent
	case "image":
		if msg.ImageURL != "" {
			media = append(media, msg.ImageURL)
		}
	case "mixed":
		for _, item := range msg.MixedItems {
			if item.MsgType == "text" {
				content += item.Content
			}
			if item.MsgType == "image" && item.ImageURL != "" {
				media = append(media, item.ImageURL)
			}
		}
	}

	// Use webhook_url from the message itself (per-message response URL).
	responseURL := msg.WebhookURL
	if responseURL == "" {
		responseURL = c.config.WebhookURL
	}

	metadata := map[string]string{
		"msg_type":     msg.MsgType,
		"msg_id":       msg.MsgID,
		"platform":     "wecom",
		"response_url": responseURL,
	}
	if isGroup {
		metadata["peer_kind"] = "group"
		metadata["chat_id"] = msg.ChatID
		metadata["sender_id"] = senderID
	} else {
		metadata["peer_kind"] = "direct"
	}
	if msg.FromName != "" {
		metadata["sender_name"] = msg.FromName
	}

	slog.Info("wecom: processing message",
		"msg_id", msg.MsgID, "msg_type", msg.MsgType,
		"from", senderID, "chat_type", msg.ChatType)

	userID := c.userID
	if userID == "" {
		userID = senderID
	}
	_ = c.HandleMessage(ctx, c.tenantID, userID, senderID, chatID, content, media, metadata)
}

// sendReply posts a text reply to the given response URL.
func (c *WeComChannel) sendReply(ctx context.Context, responseURL, content string) error {
	reply := ReplyMessage{MsgType: "text"}
	reply.Text.Content = content

	body, err := json.Marshal(reply)
	if err != nil {
		return fmt.Errorf("wecom: marshal reply: %w", err)
	}

	timeout := c.config.ReplyTimeout
	if timeout <= 0 {
		timeout = 5
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wecom: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := channels.DefaultHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("wecom: send reply: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("wecom: read response: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("wecom: parse response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom: API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	slog.Info("wecom: reply sent", "url_len", len(responseURL), "content_len", len(content))
	return nil
}
