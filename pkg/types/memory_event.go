package types

import "time"

// MemoryEvent records a timestamped episodic event for a tenant/user.
// Stored in MySQL for structured time-range queries (unlike vector memories).
type MemoryEvent struct {
	ID        int64
	TenantID  string
	UserID    string
	Category  string // e.g. "conversation", "action", "consolidation"
	Summary   string
	CreatedAt time.Time
}
