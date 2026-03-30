package types

import "time"

// Common default identifiers used across the codebase.
const (
	DefaultTenantID = "default"
	DefaultUserID   = "user"
	StatusPublished = "published"
)

// Tenant represents a generic tenant (forum, group, org, etc.).
type Tenant struct {
	TenantID  string
	Name      string
	GroupID   string
	Config    map[string]any
	CreatedAt time.Time
}

// WorkspaceDoc represents a tenant-level workspace document.
type WorkspaceDoc struct {
	TenantID  string
	DocType   string // "IDENTITY", "SOUL", "RULES", "MEMORY"
	Content   string
	Version   int64
	UpdatedAt time.Time
}

// SkillTier defines the layer at which a skill is registered.
type SkillTier string

const (
	SkillTierBuiltin SkillTier = "builtin"
	SkillTierGlobal  SkillTier = "global"
	SkillTierGroup   SkillTier = "group"
)

// SkillRecord represents a global skill registry entry.
type SkillRecord struct {
	ID          int64
	Name        string
	Description string
	Version     string
	ObjectPath  string // BOS/S3 path
	Tier        SkillTier
	AlwaysLoad  bool
	Status      string // "draft" | "published" | "deprecated"
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TenantSkill represents a per-tenant installed skill.
type TenantSkill struct {
	TenantID    string
	SkillID     int64
	AlwaysLoad  *bool // nil=inherit default
	Config      map[string]any
	Priority    int
	InstalledAt time.Time
}

// SkillProposal represents a skill promotion request (agent-created to global).
type SkillProposal struct {
	ID         int64
	SkillName  string
	ProposedBy string
	ObjectPath string
	Status     string // "pending" | "approved" | "rejected"
	ReviewedBy string
	CreatedAt  time.Time
}

// UserWorkspaceDoc represents a user-level workspace document within a tenant.
type UserWorkspaceDoc struct {
	TenantID  string
	UserID    string
	DocType   string // "USER", "MEMORY"
	Content   string
	Version   int64
	UpdatedAt time.Time
}
