package types

import (
	"context"
	"time"
)

// Account represents a web console user account.
type Account struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	Status       string // "active", "suspended"
	Role         string // "user", "admin"
	CreatedAt    time.Time
}

// AccountTenant links an account to a tenant with a specific role.
type AccountTenant struct {
	AccountID string
	TenantID  string
	Role      string // "owner", "editor", "viewer"
	CreatedAt time.Time
}

// ChatSession represents a web console conversation session.
type ChatSession struct {
	ID         string
	TenantID   string
	AccountID  string
	AgentID    string
	Title      string
	SessionKey string // maps to the underlying ADK session key
	Pinned     bool
	Archived   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AccountStore provides CRUD operations for accounts.
type AccountStore interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	GetByEmail(ctx context.Context, email string) (*Account, error)
	Update(ctx context.Context, account *Account) error
}

// AccountTenantStore provides operations for account-tenant relationships.
type AccountTenantStore interface {
	Create(ctx context.Context, at *AccountTenant) error
	ListByAccount(ctx context.Context, accountID string) ([]*AccountTenant, error)
	HasAccess(ctx context.Context, accountID, tenantID string) (bool, error)
}

// ChatSessionStore provides CRUD operations for chat sessions.
type ChatSessionStore interface {
	Create(ctx context.Context, session *ChatSession) error
	Get(ctx context.Context, id string) (*ChatSession, error)
	// GetByAccountID fetches a session only if it belongs to the given account (prevents IDOR).
	GetByAccountID(ctx context.Context, id, accountID string) (*ChatSession, error)
	ListByAccount(ctx context.Context, accountID, tenantID string, archived bool) ([]*ChatSession, error)
	Update(ctx context.Context, session *ChatSession) error
	Delete(ctx context.Context, id string) error
}
