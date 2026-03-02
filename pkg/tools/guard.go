package tools

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/genai"

	"google.golang.org/adk/tool"
)

// innerFunctionTool is the duck-typed interface matching ADK-Go's internal
// toolinternal.FunctionTool. The runner uses type assertion against these methods.
type innerFunctionTool interface {
	tool.Tool
	Declaration() *genai.FunctionDeclaration
	Run(ctx tool.Context, args any) (map[string]any, error)
}

// wrapGuard returns the tool unchanged when no TenantStore or RateLimiter
// is configured (backward compatible). Otherwise it wraps the tool with
// per-tenant permission and rate limit checks evaluated at call time.
func wrapGuard(name string, inner tool.Tool, deps *Deps) tool.Tool {
	if deps.TenantStore == nil && deps.RateLimiter == nil {
		return inner
	}
	ft, ok := inner.(innerFunctionTool)
	if !ok {
		return inner // can't wrap non-function tools
	}
	return &guardedTool{
		inner:       ft,
		name:        name,
		tenantStore: deps.TenantStore,
		rateLimiter: deps.RateLimiter,
	}
}

// guardedTool wraps an innerFunctionTool with per-tenant permission and rate
// limit checks. It implements the same duck-typed FunctionTool interface so
// the ADK-Go runner can invoke it transparently.
type guardedTool struct {
	inner       innerFunctionTool
	name        string
	tenantStore TenantStore
	rateLimiter *TenantRateLimiter
}

// tool.Tool interface methods — delegate to inner.
func (g *guardedTool) Name() string        { return g.inner.Name() }
func (g *guardedTool) Description() string { return g.inner.Description() }
func (g *guardedTool) IsLongRunning() bool { return g.inner.IsLongRunning() }

// Declaration delegates to inner (duck-typed FunctionTool).
func (g *guardedTool) Declaration() *genai.FunctionDeclaration {
	return g.inner.Declaration()
}

// Run intercepts tool execution to check tenant permissions, skill capabilities, and rate limits.
func (g *guardedTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	tenantID := stateStr(ctx, "tenant_id")

	// Check per-tenant denied tools list.
	if g.tenantStore != nil && tenantID != "" {
		if err := g.checkPermission(ctx, tenantID); err != nil {
			return map[string]any{"error": err.Error()}, nil
		}
	}

	// Check skill-level capability restrictions.
	// The "skill_allowed_tools" state key is expected to be set by the skill
	// execution dispatcher before invoking tools within a skill context.
	// When not set (nil or empty), all tools are allowed (backward compatible).
	if allowed := stateStrSlice(ctx, "skill_allowed_tools"); len(allowed) > 0 {
		if !containsStr(allowed, g.name) {
			return map[string]any{"error": fmt.Sprintf("tool %q not in skill capabilities", g.name)}, nil
		}
	}

	// Check rate limit.
	if g.rateLimiter != nil && tenantID != "" {
		if err := g.rateLimiter.Allow(tenantID); err != nil {
			return map[string]any{"error": err.Error()}, nil
		}
	}

	return g.inner.Run(ctx, args)
}

func (g *guardedTool) checkPermission(ctx context.Context, tenantID string) error {
	tenant, err := g.tenantStore.Get(ctx, tenantID)
	if err != nil {
		slog.Warn("guard: tenant store error, denying tool call (fail-closed)", "tenant", tenantID, "err", err)
		return fmt.Errorf("tool %q: permission check failed for tenant %q: %w", g.name, tenantID, err)
	}
	if tenant == nil {
		return nil
	}
	// denied_tools may be stored as []any (from JSON) or []string.
	switch denied := tenant.Config["denied_tools"].(type) {
	case []any:
		for _, d := range denied {
			if s, ok := d.(string); ok && s == g.name {
				return fmt.Errorf("tool %q is not permitted for tenant %q", g.name, tenantID)
			}
		}
	case []string:
		for _, s := range denied {
			if s == g.name {
				return fmt.Errorf("tool %q is not permitted for tenant %q", g.name, tenantID)
			}
		}
	}
	return nil
}

// stateStrSlice reads a []string value from session state.
// Returns nil if the key is missing or not a []any / []string.
func stateStrSlice(ctx tool.Context, key string) []string {
	v, err := ctx.State().Get(key)
	if err != nil || v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// containsStr checks if a string slice contains a given value.
func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
