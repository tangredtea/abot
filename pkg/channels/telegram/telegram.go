package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"abot/pkg/channels"
	"abot/pkg/types"
)

const ChannelName = "telegram"

// TelegramChannel implements types.Channel for Telegram Bot API.
type TelegramChannel struct {
	*channels.BaseChannel
	config   TelegramConfig
	apiBase  string
	offset   int64
	tenantID string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewTelegramChannel creates a new Telegram Bot channel.
func NewTelegramChannel(cfg TelegramConfig, bus types.MessageBus) (*TelegramChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram: token is required")
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 30
	}
	tid := cfg.TenantID
	if tid == "" {
		tid = types.DefaultTenantID
	}
	return &TelegramChannel{
		BaseChannel: channels.NewBaseChannel(ChannelName, bus, cfg.AllowFrom),
		config:      cfg,
		apiBase:     "https://api.telegram.org/bot" + cfg.Token,
		tenantID:    tid,
	}, nil
}

// Start begins the long-polling loop in a goroutine.
func (c *TelegramChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}
	c.SetRunning(true)

	pctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.pollLoop(pctx)

	slog.Info("telegram: channel started", "tenant", c.tenantID)
	return nil
}

// Stop signals the poll loop to exit and waits for it to finish.
func (c *TelegramChannel) Stop(_ context.Context) error {
	if !c.IsRunning() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.SetRunning(false)
	slog.Info("telegram: channel stopped")
	return nil
}

// Send delivers an outbound message via the Telegram sendMessage API.
func (c *TelegramChannel) Send(ctx context.Context, msg types.OutboundMessage) error {
	if msg.Content == "" {
		return nil
	}
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat_id %q: %w", msg.ChatID, err)
	}
	return c.sendMessage(ctx, chatID, msg.Content)
}

// pollLoop calls getUpdates in a loop until context is cancelled.
func (c *TelegramChannel) pollLoop(ctx context.Context) {
	defer c.wg.Done()

	client := &http.Client{Timeout: time.Duration(c.config.PollTimeout+5) * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := c.getUpdates(ctx, client)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("telegram: getUpdates error", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, u := range updates {
			c.offset = u.UpdateID + 1
			if u.Message != nil {
				c.processUpdate(ctx, u.Message)
			}
		}
	}
}

// getUpdates calls the Telegram getUpdates API.
func (c *TelegramChannel) getUpdates(ctx context.Context, client *http.Client) ([]Update, error) {
	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=%d", c.apiBase, c.offset, c.config.PollTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: create getUpdates request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getUpdates: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result getUpdatesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API returned ok=false")
	}
	return result.Result, nil
}

// processUpdate extracts content from a Telegram message and publishes to the bus.
func (c *TelegramChannel) processUpdate(ctx context.Context, msg *Message) {
	if msg.From == nil || msg.From.IsBot {
		return
	}

	senderID := strconv.FormatInt(msg.From.ID, 10)
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	userID := senderID

	content := msg.Text
	var media []string
	if len(msg.Photo) > 0 {
		// Use the largest photo (last in array).
		media = append(media, msg.Photo[len(msg.Photo)-1].FileID)
	}

	if content == "" && len(media) == 0 {
		return
	}

	metadata := map[string]string{
		"platform":   "telegram",
		"chat_type":  msg.Chat.Type,
		"message_id": strconv.FormatInt(msg.MessageID, 10),
	}
	if msg.From.Username != "" {
		metadata["username"] = msg.From.Username
	}

	_ = c.HandleMessage(ctx, c.tenantID, userID, senderID, chatID, content, media, metadata)
}

// sendMessage calls the Telegram sendMessage API.
func (c *TelegramChannel) sendMessage(ctx context.Context, chatID int64, text string) error {
	payload, err := json.Marshal(sendMessageRequest{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		return fmt.Errorf("telegram: marshal: %w", err)
	}

	url := c.apiBase + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}

	var result apiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("telegram: parse response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram: API error: %s", result.Description)
	}
	return nil
}
