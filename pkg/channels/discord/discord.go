package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"abot/pkg/channels"
	"abot/pkg/types"
)

const (
	ChannelName = "discord"
	gatewayURL  = "wss://gateway.discord.gg/?v=10&encoding=json"
	apiBase     = "https://discord.com/api/v10"

	// Gateway opcodes.
	opDispatch  = 0
	opHeartbeat = 1
	opIdentify  = 2
	opHello     = 10
	opHeartbeatACK = 11

	// Intents: GUILDS | GUILD_MESSAGES | MESSAGE_CONTENT | DIRECT_MESSAGES.
	defaultIntents = 1 | 512 | 32768 | 4096

	// reconnectDelay is the wait time before reconnecting after a gateway error.
	reconnectDelay = 5 * time.Second
)

// DiscordChannel implements types.Channel for Discord Bot.
type DiscordChannel struct {
	*channels.BaseChannel
	config   DiscordConfig
	tenantID string
	seq      atomic.Pointer[int64]
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewDiscordChannel creates a new Discord Bot channel.
func NewDiscordChannel(cfg DiscordConfig, bus types.MessageBus) (*DiscordChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord: token is required")
	}
	tid := cfg.TenantID
	if tid == "" {
		tid = types.DefaultTenantID
	}
	return &DiscordChannel{
		BaseChannel: channels.NewBaseChannel(ChannelName, bus, cfg.AllowFrom),
		config:      cfg,
		tenantID:    tid,
	}, nil
}

// Start connects to the Discord Gateway and begins listening.
func (c *DiscordChannel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}
	c.SetRunning(true)

	gctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.gatewayLoop(gctx)

	slog.Info("discord: channel started", "tenant", c.tenantID)
	return nil
}

// Stop disconnects from the Gateway.
func (c *DiscordChannel) Stop(_ context.Context) error {
	if !c.IsRunning() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.SetRunning(false)
	slog.Info("discord: channel stopped")
	return nil
}

// Send delivers an outbound message via the Discord REST API.
func (c *DiscordChannel) Send(ctx context.Context, msg types.OutboundMessage) error {
	if msg.Content == "" {
		return nil
	}
	return c.sendMessage(ctx, msg.ChatID, msg.Content)
}

// gatewayLoop connects to Discord Gateway, identifies, and listens for events.
func (c *DiscordChannel) gatewayLoop(ctx context.Context) {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("discord: gatewayLoop panic", "recover", r)
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		if err := c.runGateway(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("discord: gateway error, reconnecting", "err", err)
			time.Sleep(reconnectDelay)
		}
	}
}

// runGateway handles a single Gateway WebSocket session.
func (c *DiscordChannel) runGateway(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Read HELLO.
	var hello gatewayPayload
	if err := conn.ReadJSON(&hello); err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	if hello.Op != opHello {
		return fmt.Errorf("expected hello, got op=%d", hello.Op)
	}
	var helloData helloPayload
	_ = json.Unmarshal(hello.D, &helloData)

	// Send IDENTIFY.
	identify := gatewayPayload{
		Op: opIdentify,
	}
	idData, _ := json.Marshal(identifyPayload{
		Token:   c.config.Token,
		Intents: defaultIntents,
		Properties: map[string]string{
			"os":      "linux",
			"browser": "abot",
			"device":  "abot",
		},
	})
	identify.D = idData
	if err := conn.WriteJSON(identify); err != nil {
		return fmt.Errorf("send identify: %w", err)
	}

	// Start heartbeat.
	hbInterval := time.Duration(helloData.HeartbeatInterval) * time.Millisecond
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go c.heartbeatLoop(hbCtx, conn, hbInterval)

	// Read events.
	return c.readEvents(ctx, conn)
}

// heartbeatLoop sends periodic heartbeats to keep the Gateway alive.
func (c *DiscordChannel) heartbeatLoop(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb := gatewayPayload{Op: opHeartbeat}
			if s := c.seq.Load(); s != nil {
				d, _ := json.Marshal(*s)
				hb.D = d
			}
			if err := conn.WriteJSON(hb); err != nil {
				slog.Warn("discord: heartbeat write error", "err", err)
				return
			}
		}
	}
}

// readEvents reads dispatch events from the Gateway connection.
func (c *DiscordChannel) readEvents(ctx context.Context, conn *websocket.Conn) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		var payload gatewayPayload
		if err := conn.ReadJSON(&payload); err != nil {
			return fmt.Errorf("read event: %w", err)
		}

		if payload.S != nil {
			c.seq.Store(payload.S)
		}

		if payload.Op != opDispatch {
			continue
		}

		if payload.T == "MESSAGE_CREATE" {
			var msg discordMessage
			if err := json.Unmarshal(payload.D, &msg); err != nil {
				slog.Warn("discord: parse message", "err", err)
				continue
			}
			c.processMessage(ctx, msg)
		}
	}
}

// processMessage extracts content from a Discord message and publishes to the bus.
func (c *DiscordChannel) processMessage(ctx context.Context, msg discordMessage) {
	if msg.Author.Bot {
		return
	}
	if c.config.GuildID != "" && msg.GuildID != c.config.GuildID {
		return
	}

	senderID := msg.Author.ID
	metadata := map[string]string{
		"platform":   "discord",
		"message_id": msg.ID,
		"username":   msg.Author.Username,
	}
	if msg.GuildID != "" {
		metadata["guild_id"] = msg.GuildID
	}

	_ = c.HandleMessage(ctx, c.tenantID, senderID, senderID, msg.ChannelID, msg.Content, nil, metadata)
}

// sendMessage posts a message to a Discord channel via REST API.
func (c *DiscordChannel) sendMessage(ctx context.Context, channelID, text string) error {
	url := fmt.Sprintf("%s/channels/%s/messages", apiBase, channelID)

	payload, err := json.Marshal(sendMessageBody{Content: text})
	if err != nil {
		return fmt.Errorf("discord: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("discord: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+c.config.Token)

	resp, err := channels.DefaultHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord: API error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
