package tools

import (
	"context"
	"fmt"

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

// Run intercepts tool execution to check tenant permissions and rate limits.
func (g *guardedTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	tenantID := stateStr(ctx, "tenant_id")

	// Check per-tenant denied tools list.
	if g.tenantStore != nil && tenantID != "" {
		if err := g.checkPermission(ctx, tenantID); err != nil {
			return map[string]any{"error": err.Error()}, nil
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
	if err != nil || tenant == nil {
		return nil // unknown tenant = allow (fail open)
	}
	denied, _ := tenant.Config["denied_tools"].([]any)
	for _, d := range denied {
		if s, ok := d.(string); ok && s == g.name {
			return fmt.Errorf("tool %q is not permitted for tenant %q", g.name, tenantID)
		}
	}
	return nil
}
