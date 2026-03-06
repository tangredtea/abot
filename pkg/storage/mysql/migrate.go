package mysql

import (
	"database/sql"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// --- GORM Models ---

// TenantModel maps to the tenants table.
type TenantModel struct {
	TenantID  string    `gorm:"column:tenant_id;type:varchar(128);primaryKey"`
	Name      string    `gorm:"column:name;type:varchar(255)"`
	GroupID   string    `gorm:"column:group_id;type:varchar(128);index"`
	Config    JSON      `gorm:"column:config;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (TenantModel) TableName() string { return "tenants" }

// WorkspaceDocModel maps to the workspace_docs table.
type WorkspaceDocModel struct {
	TenantID  string    `gorm:"column:tenant_id;type:varchar(128);primaryKey"`
	DocType   string    `gorm:"column:doc_type;type:varchar(128);primaryKey"`
	Content   string    `gorm:"column:content;type:longtext"`
	Version   int64     `gorm:"column:version"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (WorkspaceDocModel) TableName() string { return "workspace_docs" }

// UserWorkspaceDocModel maps to the user_workspace_docs table.
type UserWorkspaceDocModel struct {
	TenantID  string    `gorm:"column:tenant_id;type:varchar(128);primaryKey"`
	UserID    string    `gorm:"column:user_id;type:varchar(128);primaryKey;index"`
	DocType   string    `gorm:"column:doc_type;type:varchar(128);primaryKey"`
	Content   string    `gorm:"column:content;type:longtext"`
	Version   int64     `gorm:"column:version"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserWorkspaceDocModel) TableName() string { return "user_workspace_docs" }

// SkillRecordModel maps to the skill_records table.
type SkillRecordModel struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `gorm:"column:name;type:varchar(255);uniqueIndex"`
	Description string    `gorm:"column:description;type:text"`
	Version     string    `gorm:"column:version"`
	ObjectPath  string    `gorm:"column:object_path"`
	Tier        string    `gorm:"column:tier;index:idx_tier_status"`
	AlwaysLoad  bool      `gorm:"column:always_load"`
	Status      string    `gorm:"column:status;index:idx_tier_status"`
	Metadata    JSON      `gorm:"column:metadata;type:json"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (SkillRecordModel) TableName() string { return "skill_records" }

// TenantSkillModel maps to the tenant_skills table.
type TenantSkillModel struct {
	TenantID    string       `gorm:"column:tenant_id;type:varchar(128);primaryKey;index"`
	SkillID     int64        `gorm:"column:skill_id;primaryKey"`
	AlwaysLoad  sql.NullBool `gorm:"column:always_load"`
	Config      JSON         `gorm:"column:config;type:json"`
	Priority    int          `gorm:"column:priority"`
	InstalledAt time.Time    `gorm:"column:installed_at;autoCreateTime"`
}

func (TenantSkillModel) TableName() string { return "tenant_skills" }

// SkillProposalModel maps to the skill_proposals table.
type SkillProposalModel struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SkillName  string    `gorm:"column:skill_name"`
	ProposedBy string    `gorm:"column:proposed_by"`
	ObjectPath string    `gorm:"column:object_path"`
	Status     string    `gorm:"column:status"`
	ReviewedBy string    `gorm:"column:reviewed_by"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (SkillProposalModel) TableName() string { return "skill_proposals" }

// CronJobModel maps to the cron_jobs table.
type CronJobModel struct {
	ID             string    `gorm:"column:id;type:varchar(128);primaryKey"`
	TenantID       string    `gorm:"column:tenant_id;type:varchar(128);index:idx_tenant_enabled"`
	UserID         string    `gorm:"column:user_id;type:varchar(128)"`
	Name           string    `gorm:"column:name"`
	Enabled        bool      `gorm:"column:enabled;index:idx_tenant_enabled"`
	Schedule       JSON      `gorm:"column:schedule;type:json"`
	Message        string    `gorm:"column:message;type:text"`
	Channel        string    `gorm:"column:channel"`
	ChatID         string    `gorm:"column:chat_id"`
	DeleteAfterRun bool      `gorm:"column:delete_after_run"`
	State          JSON      `gorm:"column:state;type:json"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (CronJobModel) TableName() string { return "cron_jobs" }

// AgentRouteModel maps to the agent_routes table.
type AgentRouteModel struct {
	AgentID   string `gorm:"column:agent_id;type:varchar(128);primaryKey"`
	Channel   string `gorm:"column:channel"`
	ChatID    string `gorm:"column:chat_id"`
	AccountID string `gorm:"column:account_id;type:varchar(128)"`
	PeerKind  string `gorm:"column:peer_kind;type:varchar(64)"`
	PeerID    string `gorm:"column:peer_id;type:varchar(128)"`
	GuildID   string `gorm:"column:guild_id;type:varchar(128)"`
	TeamID    string `gorm:"column:team_id;type:varchar(128)"`
	Priority  int    `gorm:"column:priority"`
	IsDefault bool   `gorm:"column:is_default"`
}

func (AgentRouteModel) TableName() string { return "agent_routes" }

// MemoryEventModel maps to the memory_events table.
type MemoryEventModel struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID  string    `gorm:"column:tenant_id;type:varchar(128);index:idx_mem_evt_tenant_user"`
	UserID    string    `gorm:"column:user_id;type:varchar(128);index:idx_mem_evt_tenant_user"`
	Category  string    `gorm:"column:category;type:varchar(64)"`
	Summary   string    `gorm:"column:summary;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_mem_evt_created"`
}

func (MemoryEventModel) TableName() string { return "memory_events" }

// AccountModel maps to the accounts table.
type AccountModel struct {
	ID           string    `gorm:"column:id;type:varchar(128);primaryKey"`
	Email        string    `gorm:"column:email;type:varchar(255);uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(255)"`
	DisplayName  string    `gorm:"column:display_name;type:varchar(255)"`
	Status       string    `gorm:"column:status;type:varchar(32);default:active"`
	Role         string    `gorm:"column:role;type:varchar(32);default:user"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (AccountModel) TableName() string { return "accounts" }

// AccountTenantModel maps to the account_tenants table.
type AccountTenantModel struct {
	AccountID string    `gorm:"column:account_id;type:varchar(128);primaryKey"`
	TenantID  string    `gorm:"column:tenant_id;type:varchar(128);primaryKey"`
	Role      string    `gorm:"column:role;type:varchar(32);default:owner"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (AccountTenantModel) TableName() string { return "account_tenants" }

// ChatSessionModel maps to the chat_sessions table.
type ChatSessionModel struct {
	ID         string    `gorm:"column:id;type:varchar(128);primaryKey"`
	TenantID   string    `gorm:"column:tenant_id;type:varchar(128);index:idx_cs_account_tenant"`
	AccountID  string    `gorm:"column:account_id;type:varchar(128);index:idx_cs_account_tenant"`
	AgentID    string    `gorm:"column:agent_id;type:varchar(128)"`
	Title      string    `gorm:"column:title;type:varchar(512)"`
	SessionKey string    `gorm:"column:session_key;type:varchar(512)"`
	Pinned     bool      `gorm:"column:pinned;default:false"`
	Archived   bool      `gorm:"column:archived;default:false"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ChatSessionModel) TableName() string { return "chat_sessions" }

// AgentDefinitionModel maps to the agent_definitions table.
type AgentDefinitionModel struct {
	ID          string    `gorm:"column:id;type:varchar(128);primaryKey"`
	TenantID    string    `gorm:"column:tenant_id;type:varchar(128);index:idx_agent_tenant"`
	Name        string    `gorm:"column:name;type:varchar(255)"`
	Description string    `gorm:"column:description;type:text"`
	Avatar      string    `gorm:"column:avatar;type:varchar(32)"`
	Model       string    `gorm:"column:model;type:varchar(128)"`
	Provider    string    `gorm:"column:provider;type:varchar(128)"`
	Status      string    `gorm:"column:status;type:varchar(32);default:active"`
	Config      JSON      `gorm:"column:config;type:json"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (AgentDefinitionModel) TableName() string { return "agent_definitions" }

// AgentChannelModel maps to the agent_channels table.
type AgentChannelModel struct {
	AgentID   string    `gorm:"column:agent_id;type:varchar(128);primaryKey"`
	Channel   string    `gorm:"column:channel;type:varchar(64);primaryKey"`
	Enabled   bool      `gorm:"column:enabled;default:true"`
	Config    JSON      `gorm:"column:config;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (AgentChannelModel) TableName() string { return "agent_channels" }

// --- JSON helper type for GORM ---

// JSON is a json.RawMessage wrapper that implements GORM's Scanner/Valuer.
type JSON json.RawMessage

func (j JSON) Value() (any, error) {
	if len(j) == 0 || string(j) == "null" {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = JSON(v)
	case string:
		*j = JSON(v)
	}
	return nil
}

// AutoMigrate creates all tables.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&TenantModel{},
		&WorkspaceDocModel{},
		&UserWorkspaceDocModel{},
		&SkillRecordModel{},
		&TenantSkillModel{},
		&SkillProposalModel{},
		&CronJobModel{},
		&AgentRouteModel{},
		&MemoryEventModel{},
		&AccountModel{},
		&AccountTenantModel{},
		&ChatSessionModel{},
		&AgentDefinitionModel{},
		&AgentChannelModel{},
	)
}
