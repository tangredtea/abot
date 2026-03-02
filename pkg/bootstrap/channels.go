package bootstrap

import (
	"fmt"
	"log/slog"

	"abot/pkg/agent"
	"abot/pkg/channels/discord"
	"abot/pkg/channels/feishu"
	"abot/pkg/channels/telegram"
	"abot/pkg/channels/wecom"
	"abot/pkg/types"
)

// NewChannels creates channel adapters from config.
func NewChannels(cfg *agent.Config, msgBus types.MessageBus) (map[string]types.Channel, error) {
	chans := make(map[string]types.Channel)

	// WeCom channel
	if cfg.WeCom.Token != "" {
		wc, err := wecom.NewWeComChannel(wecom.WeComConfig{
			Token:          cfg.WeCom.Token,
			EncodingAESKey: cfg.WeCom.EncodingAESKey,
			WebhookURL:     cfg.WeCom.WebhookURL,
			WebhookHost:    cfg.WeCom.WebhookHost,
			WebhookPort:    cfg.WeCom.WebhookPort,
			WebhookPath:    cfg.WeCom.WebhookPath,
			ReplyTimeout:   cfg.WeCom.ReplyTimeout,
			AllowFrom:      cfg.WeCom.AllowFrom,
			TenantID:       cfg.WeCom.TenantID,
			UserID:         cfg.WeCom.UserID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("wecom channel: %w", err)
		}
		chans[wecom.ChannelName] = wc
		slog.Info("wecom channel configured")
	}

	// Telegram channel
	if cfg.Telegram.Token != "" {
		tc, err := telegram.NewTelegramChannel(telegram.TelegramConfig{
			Token:       cfg.Telegram.Token,
			AllowFrom:   cfg.Telegram.AllowFrom,
			TenantID:    cfg.Telegram.TenantID,
			PollTimeout: cfg.Telegram.PollTimeout,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("telegram channel: %w", err)
		}
		chans[telegram.ChannelName] = tc
		slog.Info("telegram channel configured")
	}

	// Discord channel
	if cfg.Discord.Token != "" {
		dc, err := discord.NewDiscordChannel(discord.DiscordConfig{
			Token:     cfg.Discord.Token,
			AllowFrom: cfg.Discord.AllowFrom,
			TenantID:  cfg.Discord.TenantID,
			GuildID:   cfg.Discord.GuildID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("discord channel: %w", err)
		}
		chans[discord.ChannelName] = dc
		slog.Info("discord channel configured")
	}

	// Feishu channel
	if cfg.Feishu.AppID != "" {
		fc, err := feishu.NewFeishuChannel(feishu.FeishuConfig{
			AppID:             cfg.Feishu.AppID,
			AppSecret:         cfg.Feishu.AppSecret,
			VerificationToken: cfg.Feishu.VerificationToken,
			EncryptKey:        cfg.Feishu.EncryptKey,
			WebhookHost:       cfg.Feishu.WebhookHost,
			WebhookPort:       cfg.Feishu.WebhookPort,
			WebhookPath:       cfg.Feishu.WebhookPath,
			AllowFrom:         cfg.Feishu.AllowFrom,
			TenantID:          cfg.Feishu.TenantID,
		}, msgBus)
		if err != nil {
			return nil, fmt.Errorf("feishu channel: %w", err)
		}
		chans[feishu.ChannelName] = fc
		slog.Info("feishu channel configured")
	}

	return chans, nil
}
