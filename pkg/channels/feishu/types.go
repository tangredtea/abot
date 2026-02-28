package feishu

// FeishuConfig holds configuration for the Feishu Bot channel.
type FeishuConfig struct {
	AppID             string   `yaml:"app_id"`
	AppSecret         string   `yaml:"app_secret"`
	VerificationToken string   `yaml:"verification_token"`
	EncryptKey        string   `yaml:"encrypt_key,omitempty"`
	WebhookHost       string   `yaml:"webhook_host"`
	WebhookPort       int      `yaml:"webhook_port"`
	WebhookPath       string   `yaml:"webhook_path"`
	AllowFrom         []string `yaml:"allow_from,omitempty"`
	TenantID          string   `yaml:"tenant_id"`
}
