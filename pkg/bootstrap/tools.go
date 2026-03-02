package bootstrap

import (
	"fmt"

	"abot/pkg/agent"
	"abot/pkg/storage/objectstore"
	"abot/pkg/tools"
	"abot/pkg/types"
)

// BuildLightweightTools builds tools that don't require database.
// Used by abot-agent.
// Note: Currently uses BuildAllTools but with nil database stores.
// Tools that require database will be skipped automatically.
func BuildLightweightTools(cfg *agent.Config) ([]any, error) {
	// Object store
	objStoreDir := cfg.ObjectStore.Dir
	if objStoreDir == "" {
		objStoreDir = "data/objects"
	}
	objStore := objectstore.NewLocalStore(objStoreDir)

	// Lightweight tools deps (no database stores)
	toolsDeps := &tools.Deps{
		ObjectStore:         objStore,
		WorkspaceDir:        "workspace",
		DenyPatterns:        append([]string{".env", "*.key", "*.pem"}, cfg.Sandbox.ExtraDenyPatterns...),
		RestrictToWorkspace: cfg.Sandbox.RestrictToWorkspace,
		AllowedPaths:        cfg.Sandbox.AllowedPaths,
	}

	// Wire ExecLimits from sandbox config
	if cfg.Sandbox.ExecMemoryMB > 0 || cfg.Sandbox.ExecCPUSeconds > 0 ||
		cfg.Sandbox.ExecFileSizeMB > 0 || cfg.Sandbox.ExecNProc > 0 {
		toolsDeps.ExecLimits = &tools.ExecLimits{
			MemoryMB:   cfg.Sandbox.ExecMemoryMB,
			CPUSeconds: cfg.Sandbox.ExecCPUSeconds,
			FileSizeMB: cfg.Sandbox.ExecFileSizeMB,
			NProc:      cfg.Sandbox.ExecNProc,
		}
	}

	// Wire SandboxOpts for Landlock kernel-level filesystem sandbox
	if cfg.Sandbox.Level != "" && cfg.Sandbox.Level != "none" {
		toolsDeps.SandboxOpts = &tools.SandboxOpts{
			Level:        tools.SandboxLevel(cfg.Sandbox.Level),
			HelperBinary: cfg.Sandbox.SandboxBinary,
		}
	}

	// Wire per-tenant rate limiter
	if cfg.Sandbox.RateLimit > 0 {
		burst := cfg.Sandbox.RateBurst
		if burst <= 0 {
			burst = 10
		}
		toolsDeps.RateLimiter = tools.NewTenantRateLimiter(cfg.Sandbox.RateLimit, burst)
	}

	// Build all tools (tools requiring database will return nil and be skipped)
	builtTools, err := tools.BuildAllTools(toolsDeps)
	if err != nil {
		return nil, fmt.Errorf("build lightweight tools: %w", err)
	}

	toolsAsAny := make([]any, len(builtTools))
	for i, t := range builtTools {
		toolsAsAny[i] = t
	}

	return toolsAsAny, nil
}

// BuildFullTools builds all tools including database-dependent ones.
// Used by abot-server and abot-web.
func BuildFullTools(cfg *agent.Config, stores *StoreBundle, msgBus types.MessageBus) ([]any, error) {
	// Object store
	objStoreDir := cfg.ObjectStore.Dir
	if objStoreDir == "" {
		objStoreDir = "data/objects"
	}
	objStore := objectstore.NewLocalStore(objStoreDir)

	// Full tools deps
	toolsDeps := &tools.Deps{
		Bus:                 msgBus,
		WorkspaceStore:      stores.Workspace,
		UserWorkspaceStore:  stores.UserWorkspace,
		SkillRegistryStore:  stores.SkillRegistry,
		TenantSkillStore:    stores.TenantSkill,
		SchedulerStore:      stores.Scheduler,
		ObjectStore:         objStore,
		ProposalStore:       stores.Proposal,
		WorkspaceDir:        "workspace",
		DenyPatterns:        append([]string{".env", "*.key", "*.pem"}, cfg.Sandbox.ExtraDenyPatterns...),
		RestrictToWorkspace: cfg.Sandbox.RestrictToWorkspace,
		AllowedPaths:        cfg.Sandbox.AllowedPaths,
	}

	// Wire ExecLimits from sandbox config
	if cfg.Sandbox.ExecMemoryMB > 0 || cfg.Sandbox.ExecCPUSeconds > 0 ||
		cfg.Sandbox.ExecFileSizeMB > 0 || cfg.Sandbox.ExecNProc > 0 {
		toolsDeps.ExecLimits = &tools.ExecLimits{
			MemoryMB:   cfg.Sandbox.ExecMemoryMB,
			CPUSeconds: cfg.Sandbox.ExecCPUSeconds,
			FileSizeMB: cfg.Sandbox.ExecFileSizeMB,
			NProc:      cfg.Sandbox.ExecNProc,
		}
	}

	// Wire SandboxOpts for Landlock kernel-level filesystem sandbox
	if cfg.Sandbox.Level != "" && cfg.Sandbox.Level != "none" {
		toolsDeps.SandboxOpts = &tools.SandboxOpts{
			Level:        tools.SandboxLevel(cfg.Sandbox.Level),
			HelperBinary: cfg.Sandbox.SandboxBinary,
		}
	}

	// Wire per-tenant rate limiter
	if cfg.Sandbox.RateLimit > 0 {
		burst := cfg.Sandbox.RateBurst
		if burst <= 0 {
			burst = 10
		}
		toolsDeps.RateLimiter = tools.NewTenantRateLimiter(cfg.Sandbox.RateLimit, burst)
	}

	// Wire tenant store for per-tenant tool permission checks
	toolsDeps.TenantStore = stores.Tenant

	// Build all tools
	builtTools, err := tools.BuildAllTools(toolsDeps)
	if err != nil {
		return nil, fmt.Errorf("build tools: %w", err)
	}

	toolsAsAny := make([]any, len(builtTools))
	for i, t := range builtTools {
		toolsAsAny[i] = t
	}

	return toolsAsAny, nil
}
