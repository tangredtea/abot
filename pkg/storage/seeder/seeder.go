// Package seeder provides idempotent database seeding for default tenants and workspace docs.
package seeder

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"abot/pkg/types"
)

// Seed ensures the default tenant, workspace docs, and test data exist.
// Idempotent — skips records that already exist.
// The caller is responsible for wrapping in a transaction if needed.
func Seed(ctx context.Context, ts types.TenantStore, ws types.WorkspaceStore, uws types.UserWorkspaceStore) error {
	if err := seedDefaultTenant(ctx, ts, ws, uws); err != nil {
		return fmt.Errorf("default tenant: %w", err)
	}
	if err := seedTestTenant(ctx, ts, ws, uws); err != nil {
		return fmt.Errorf("test tenant: %w", err)
	}
	return nil
}

func seedDefaultTenant(ctx context.Context, ts types.TenantStore, ws types.WorkspaceStore, uws types.UserWorkspaceStore) error {
	tenantID := types.DefaultTenantID
	userID := types.DefaultUserID

	if _, err := ts.Get(ctx, tenantID); err != nil {
		if err := ts.Put(ctx, &types.Tenant{
			TenantID: tenantID,
			Name:     "Default Tenant",
		}); err != nil {
			return fmt.Errorf("seed tenant: %w", err)
		}
		slog.Info("seed: created default tenant", "tenant", tenantID)
	}

	if err := seedWorkspaceDocs(ctx, ws, tenantID); err != nil {
		return err
	}
	return seedUserDocs(ctx, uws, tenantID, userID)
}

func seedWorkspaceDocs(ctx context.Context, ws types.WorkspaceStore, tenantID string) error {
	docFiles := map[string]string{
		"IDENTITY":  "workspace/IDENTITY.md",
		"SOUL":      "workspace/SOUL.md",
		"AGENT":     "workspace/AGENT.md",
		"TOOLS":     "workspace/TOOLS.md",
		"RULES":     "workspace/RULES.md",
		"HEARTBEAT": "workspace/HEARTBEAT.md",
	}
	for docType, path := range docFiles {
		if _, err := ws.Get(ctx, tenantID, docType); err == nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("seed: skip workspace doc", "doc_type", docType, "err", err)
			continue
		}
		if err := ws.Put(ctx, &types.WorkspaceDoc{
			TenantID: tenantID,
			DocType:  docType,
			Content:  string(data),
			Version:  1,
		}); err != nil {
			return fmt.Errorf("seed doc %s: %w", docType, err)
		}
		slog.Info("seed: created workspace doc", "doc_type", docType)
	}
	return nil
}

func seedUserDocs(ctx context.Context, uws types.UserWorkspaceStore, tenantID, userID string) error {
	userDocFiles := map[string]string{
		"USER":        "workspace/USER.md",
		"EXPERIMENTS": "workspace/EXPERIMENTS.md",
		"NOTES":       "workspace/NOTES.md",
	}
	for docType, path := range userDocFiles {
		if _, err := uws.Get(ctx, tenantID, userID, docType); err == nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("seed: skip user doc", "doc_type", docType, "err", err)
			continue
		}
		if err := uws.Put(ctx, &types.UserWorkspaceDoc{
			TenantID: tenantID,
			UserID:   userID,
			DocType:  docType,
			Content:  string(data),
			Version:  1,
		}); err != nil {
			return fmt.Errorf("seed user doc %s: %w", docType, err)
		}
		slog.Info("seed: created user doc", "doc_type", docType)
	}
	return nil
}

func seedTestTenant(ctx context.Context, ts types.TenantStore, ws types.WorkspaceStore, uws types.UserWorkspaceStore) error {
	const tid = "test-corp"

	if _, err := ts.Get(ctx, tid); err == nil {
		return nil
	}

	if err := ts.Put(ctx, &types.Tenant{
		TenantID: tid,
		Name:     "Test Corporation",
	}); err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	slog.Info("seed: created test tenant", "tenant", tid)

	tenantDocs := map[string]string{
		"IDENTITY": "你是 Test Corp 的专属助手「小测」。你说话简洁、幽默，喜欢用比喻解释技术概念。",
		"RULES":    "1. 始终用中文回复\n2. 回答控制在 3 句话以内\n3. 遇到不确定的问题要诚实说不知道",
	}
	for docType, content := range tenantDocs {
		if err := ws.Put(ctx, &types.WorkspaceDoc{
			TenantID: tid,
			DocType:  docType,
			Content:  content,
			Version:  1,
		}); err != nil {
			return fmt.Errorf("seed doc %s: %w", docType, err)
		}
	}

	users := []struct {
		id, profile string
	}{
		{"alice", "Alice 是后端工程师，熟悉 Go 和 Rust，偏好简洁的代码风格。"},
		{"bob", "Bob 是产品经理，关注用户体验，喜欢用数据说话。"},
	}
	for _, u := range users {
		if err := uws.Put(ctx, &types.UserWorkspaceDoc{
			TenantID: tid,
			UserID:   u.id,
			DocType:  "USER",
			Content:  u.profile,
			Version:  1,
		}); err != nil {
			return fmt.Errorf("seed user %s: %w", u.id, err)
		}
		slog.Info("seed: created user profile", "tenant", tid, "user", u.id)
	}

	return nil
}
