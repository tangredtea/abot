package types

import "context"

// AllowlistStore persists sender allowlist rules (Web mode).
type AllowlistStore interface {
	GetEntry(ctx context.Context, tenantID, chatID string) (*AllowlistEntry, error)
	SetEntry(ctx context.Context, tenantID, chatID string, entry *AllowlistEntry) error
	DeleteEntry(ctx context.Context, tenantID, chatID string) error
	ListEntries(ctx context.Context, tenantID string) (map[string]AllowlistEntry, error)
}
