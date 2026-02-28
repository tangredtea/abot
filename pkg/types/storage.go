package types

import (
	"context"
	"io"
	"time"
)

// TenantStore provides CRUD operations for tenants.
type TenantStore interface {
	Get(ctx context.Context, tenantID string) (*Tenant, error)
	Put(ctx context.Context, tenant *Tenant) error
	List(ctx context.Context, groupID string) ([]*Tenant, error)
	Delete(ctx context.Context, tenantID string) error
}

// WorkspaceStore provides CRUD operations for tenant-level workspace documents.
type WorkspaceStore interface {
	Get(ctx context.Context, tenantID string, docType string) (*WorkspaceDoc, error)
	Put(ctx context.Context, doc *WorkspaceDoc) error
	List(ctx context.Context, tenantID string) ([]*WorkspaceDoc, error)
	Delete(ctx context.Context, tenantID string, docType string) error
}

// UserWorkspaceStore provides CRUD operations for user-level workspace documents within a tenant.
type UserWorkspaceStore interface {
	Get(ctx context.Context, tenantID, userID, docType string) (*UserWorkspaceDoc, error)
	Put(ctx context.Context, doc *UserWorkspaceDoc) error
	List(ctx context.Context, tenantID, userID string) ([]*UserWorkspaceDoc, error)
	Delete(ctx context.Context, tenantID, userID, docType string) error
}

// SkillRegistryStore provides CRUD operations for the global skill registry.
type SkillRegistryStore interface {
	Get(ctx context.Context, name string) (*SkillRecord, error)
	GetByID(ctx context.Context, id int64) (*SkillRecord, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*SkillRecord, error)
	List(ctx context.Context, opts SkillListOpts) ([]*SkillRecord, error)
	Put(ctx context.Context, record *SkillRecord) error
	Delete(ctx context.Context, name string) error
}

// SkillListOpts holds query options for listing skills.
type SkillListOpts struct {
	Tier   SkillTier
	Status string
}

// TenantSkillStore provides CRUD operations for per-tenant installed skill relationships.
type TenantSkillStore interface {
	Install(ctx context.Context, ts *TenantSkill) error
	Uninstall(ctx context.Context, tenantID string, skillID int64) error
	ListInstalled(ctx context.Context, tenantID string) ([]*TenantSkill, error)
	UpdateConfig(ctx context.Context, tenantID string, skillID int64, config map[string]any) error
}

// ObjectStore abstracts object storage (S3-compatible).
type ObjectStore interface {
	Put(ctx context.Context, path string, data io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
}

// Embedder converts text into vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// VectorEntry represents a single vector record.
type VectorEntry struct {
	ID       string
	Vector   []float32
	Payload  map[string]any
}

// VectorSearchRequest describes a vector similarity search.
type VectorSearchRequest struct {
	Vector []float32
	Filter map[string]any
	TopK   int
}

// VectorResult is a single search hit.
type VectorResult struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// VectorStore abstracts a vector database for similarity search.
type VectorStore interface {
	EnsureCollection(ctx context.Context, collection string) error
	Upsert(ctx context.Context, collection string, entries []VectorEntry) error
	Search(ctx context.Context, collection string, req *VectorSearchRequest) ([]VectorResult, error)
	Delete(ctx context.Context, collection string, filter map[string]any) error
	// UpdatePayload patches payload fields for points matching the filter.
	// Only the keys present in payload are updated; existing keys are preserved.
	UpdatePayload(ctx context.Context, collection string, filter map[string]any, payload map[string]any) error
	Close() error
}

// SchedulerStore persists cron jobs per tenant.
type SchedulerStore interface {
	SaveJob(ctx context.Context, job *CronJob) error
	ListJobs(ctx context.Context, tenantID string) ([]*CronJob, error)
	DeleteJob(ctx context.Context, jobID string) error
	UpdateJobState(ctx context.Context, jobID string, state *CronJobState) error
}

// AgentStore persists agent definitions.
type AgentStore interface {
	Get(ctx context.Context, agentID string) (*AgentRoute, error)
	List(ctx context.Context) ([]*AgentRoute, error)
	Put(ctx context.Context, route *AgentRoute) error
	Delete(ctx context.Context, agentID string) error
}

// MemoryEventStore persists episodic event logs.
type MemoryEventStore interface {
	Add(ctx context.Context, event *MemoryEvent) error
	List(ctx context.Context, tenantID, userID string, from, to time.Time, limit int) ([]*MemoryEvent, error)
}

// SkillProposalStore provides CRUD operations for skill promotion proposals.
type SkillProposalStore interface {
	Create(ctx context.Context, proposal *SkillProposal) error
	Get(ctx context.Context, id int64) (*SkillProposal, error)
	List(ctx context.Context, status string) ([]*SkillProposal, error)
	UpdateStatus(ctx context.Context, id int64, status, reviewedBy string) error
}
