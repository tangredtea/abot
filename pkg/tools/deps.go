package tools

import (
	"context"

	"abot/pkg/types"
)

// SkillSearcher abstracts remote skill registry search (provided by skills.RegistryManager).
type SkillSearcher interface {
	SearchAll(ctx context.Context, query string, limit int) ([]SkillSearchResult, error)
}

// SkillSearchResult represents a single search hit from a remote registry.
type SkillSearchResult struct {
	Score        float64
	Slug         string
	DisplayName  string
	Summary      string
	Version      string
	RegistryName string
}

// SkillInstaller abstracts downloading and installing a skill from a remote registry.
type SkillInstaller interface {
	Install(ctx context.Context, slug, version, registryName string, objStore types.ObjectStore, objectPath string) (*SkillInstallResult, error)
}

// SkillsLoader abstracts skill loading and caching operations.
type SkillsLoader interface {
	InvalidateCache(name, version string)
}

// SkillInstallResult holds the outcome of a skill installation.
type SkillInstallResult struct {
	Version          string
	IsMalwareBlocked bool
	IsSuspicious     bool
	Summary          string
}

// SubagentSpawner abstracts synchronous and asynchronous sub-agent execution,
// decoupling the tools package from the agent package.
type SubagentSpawner interface {
	// SpawnSync executes a sub-agent task synchronously, blocking until completion.
	SpawnSync(ctx context.Context, task, agentID, channel, chatID, tenantID, userID string) (string, error)
	// SpawnAsync starts a sub-agent task asynchronously and returns the task ID immediately.
	SpawnAsync(ctx context.Context, task, agentID, channel, chatID, tenantID, userID string) (string, error)
	// GetTaskStatus returns the current status and result of a sub-task.
	GetTaskStatus(taskID string) (status, result string, found bool)
	// ListTasks returns summaries of all sub-tasks.
	ListTasks() []types.TaskSummary
}

// TenantStore reads tenant configuration for per-tenant tool permission checks.
// Implemented by storage/mysql.TenantStore.
type TenantStore interface {
	Get(ctx context.Context, tenantID string) (*types.Tenant, error)
}

// ExecLimits configures resource constraints applied to exec commands via ulimit.
// Nil means no limits (backward compatible).
type ExecLimits struct {
	MemoryMB   int // ulimit -v (virtual memory in MB), default 512
	CPUSeconds int // ulimit -t (CPU time in seconds), default 30
	FileSizeMB int // ulimit -f (max file size in MB), default 50
	NProc      int // ulimit -u (max user processes), default 64
}

// CronScheduler abstracts the cron scheduling service to avoid circular imports.
// Implemented by scheduler.CronService.
type CronScheduler interface {
	AddJob(ctx context.Context, job *types.CronJob) error
	RemoveJob(ctx context.Context, jobID string) error
	EnableJob(ctx context.Context, jobID string, enabled bool) error
	ListJobs(tenantID string) []*types.CronJob
}

// Deps holds all external dependencies needed by tool builders.
// Uses local interfaces for cross-task deps to avoid circular imports.
type Deps struct {
	Bus                 types.MessageBus
	WorkspaceStore      types.WorkspaceStore
	UserWorkspaceStore  types.UserWorkspaceStore
	SkillRegistryStore  types.SkillRegistryStore
	TenantSkillStore    types.TenantSkillStore
	SchedulerStore      types.SchedulerStore
	CronScheduler       CronScheduler // Optional; nil disables cron tool (falls back to store-only writes).
	ObjectStore         types.ObjectStore
	ProposalStore       types.SkillProposalStore
	SkillSearcher       SkillSearcher   // Provided by skills.RegistryManager.
	SkillInstaller      SkillInstaller  // Provided by skills.RegistryManager.
	SkillsLoader        SkillsLoader    // Provided by skills.SkillsLoader for cache invalidation.
	Subagent            SubagentSpawner // Optional; nil disables subagent/list_tasks tools.
	VectorStore         types.VectorStore
	Embedder            types.Embedder
	EmbeddingCache      types.EmbeddingCache // Optional; nil disables caching
	BM25Scorer          types.BM25Scorer     // Optional; nil disables BM25 scoring
	MemoryEventStore    types.MemoryEventStore
	WorkspaceDir        string
	DenyPatterns        []string
	RestrictToWorkspace bool               // when true, all file ops must stay within WorkspaceDir
	AllowedPaths        []string           // absolute paths allowed outside workspace (escape hatch)
	ExecLimits          *ExecLimits        // resource limits for exec commands (nil = no limits)
	SandboxOpts         *SandboxOpts       // Linux Landlock sandbox options (nil = no sandbox)
	TenantStore         TenantStore        // tenant config store for per-tenant tool permissions (nil = allow all)
	RateLimiter         *TenantRateLimiter // per-tenant rate limiter (nil = no limit)
}
