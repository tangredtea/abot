package bootstrap

import (
	"testing"

	"abot/pkg/agent"
	"abot/pkg/bus"
)

func TestNewChannels(t *testing.T) {
	msgBus := bus.New(100)

	tests := []struct {
		name      string
		cfg       *agent.Config
		wantCount int
		wantErr   bool
	}{
		{
			name: "no channels configured",
			cfg: &agent.Config{
				WeCom:    agent.WeComChannelConfig{Token: ""},
				Telegram: agent.TelegramChannelConfig{Token: ""},
				Discord:  agent.DiscordChannelConfig{Token: ""},
				Feishu:   agent.FeishuChannelConfig{AppID: ""},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "telegram channel configured",
			cfg: &agent.Config{
				Telegram: agent.TelegramChannelConfig{
					Token:    "test-token",
					TenantID: "test-tenant",
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple channels configured",
			cfg: &agent.Config{
				Telegram: agent.TelegramChannelConfig{
					Token:    "test-token",
					TenantID: "test-tenant",
				},
				Discord: agent.DiscordChannelConfig{
					Token:    "test-token",
					TenantID: "test-tenant",
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chans, err := NewChannels(tt.cfg, msgBus)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChannels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(chans) != tt.wantCount {
					t.Errorf("NewChannels() channel count = %v, want %v", len(chans), tt.wantCount)
				}
			}
		})
	}
}
