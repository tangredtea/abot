package types

// ToolDeps — dependency injection container for tool builders.
// All built-in tools obtain their dependencies through this container.
// Tools themselves are created via ADK-Go's functiontool.New[TArgs, TResults]().
type ToolDeps struct {
	Bus                MessageBus
	WorkspaceStore     WorkspaceStore
	UserWorkspaceStore UserWorkspaceStore
	SkillRegistryStore SkillRegistryStore
	TenantSkillStore   TenantSkillStore
	SchedulerStore     SchedulerStore
	ObjectStore        ObjectStore
	SkillsLoader       any // *skills.SkillsLoader (avoid circular import)
	RegistryManager    any // *skills.RegistryManager
	CacheLayer         any // *cache.CacheLayer
	WorkspaceDir       string
	DenyPatterns       []string
}

// ToolOutput — standard return structure for functiontool handlers.
type ToolOutput struct {
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
	ForUser string `json:"for_user,omitempty"`
}
