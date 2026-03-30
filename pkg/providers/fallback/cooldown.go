package fallback

import (
	"math"
	"sync"
	"time"

	"abot/pkg/types"
)

const defaultFailureWindow = 24 * time.Hour

// CooldownTracker manages per-provider cooldown state.
// Thread-safe. In-memory only (resets on restart).
type CooldownTracker struct {
	mu            sync.RWMutex
	entries       map[string]*cooldownEntry
	failureWindow time.Duration
	nowFunc       func() time.Time
}

type cooldownEntry struct {
	ErrorCount     int
	FailureCounts  map[types.FailoverReason]int
	CooldownEnd    time.Time            // standard cooldown expiry
	DisabledUntil  time.Time            // billing-specific disable expiry
	DisabledReason types.FailoverReason // reason for disable (billing)
	LastFailure    time.Time
}

// NewCooldownTracker creates a tracker with default 24h failure window.
func NewCooldownTracker() *CooldownTracker {
	return &CooldownTracker{
		entries:       make(map[string]*cooldownEntry),
		failureWindow: defaultFailureWindow,
		nowFunc:       time.Now,
	}
}

// SetNowFunc overrides the time source (for testing).
func (ct *CooldownTracker) SetNowFunc(fn func() time.Time) {
	ct.nowFunc = fn
}

// MarkFailure records a failure and sets exponential backoff cooldown.
func (ct *CooldownTracker) MarkFailure(provider string, reason types.FailoverReason) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	now := ct.nowFunc()
	entry := ct.getOrCreate(provider)

	// Reset if no failure in failureWindow.
	if !entry.LastFailure.IsZero() && now.Sub(entry.LastFailure) > ct.failureWindow {
		entry.ErrorCount = 0
		entry.FailureCounts = make(map[types.FailoverReason]int)
	}

	entry.ErrorCount++
	entry.FailureCounts[reason]++
	entry.LastFailure = now

	// Billing errors use a separate, longer cooldown strategy.
	if reason == types.FailoverBilling {
		billingCount := entry.FailureCounts[types.FailoverBilling]
		entry.DisabledUntil = now.Add(calculateBillingCooldown(billingCount))
		entry.DisabledReason = types.FailoverBilling
	} else {
		entry.CooldownEnd = now.Add(CalculateCooldown(entry.ErrorCount))
	}
}

// MarkSuccess resets all counters and cooldowns.
func (ct *CooldownTracker) MarkSuccess(provider string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	entry := ct.entries[provider]
	if entry == nil {
		return
	}
	entry.ErrorCount = 0
	entry.FailureCounts = make(map[types.FailoverReason]int)
	entry.CooldownEnd = time.Time{}
	entry.DisabledUntil = time.Time{}
	entry.DisabledReason = ""
}

// IsAvailable returns true if the provider is not in cooldown or disabled.
func (ct *CooldownTracker) IsAvailable(provider string) bool {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	entry := ct.entries[provider]
	if entry == nil {
		return true
	}

	now := ct.nowFunc()

	// Billing disable takes priority (longer cooldown).
	if !entry.DisabledUntil.IsZero() && now.Before(entry.DisabledUntil) {
		return false
	}

	// Standard cooldown.
	if !entry.CooldownEnd.IsZero() && now.Before(entry.CooldownEnd) {
		return false
	}

	return true
}

// CooldownRemaining returns how long until the provider exits cooldown.
func (ct *CooldownTracker) CooldownRemaining(provider string) time.Duration {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	entry := ct.entries[provider]
	if entry == nil {
		return 0
	}

	now := ct.nowFunc()
	var remaining time.Duration

	// Billing disable remaining time.
	if !entry.DisabledUntil.IsZero() && now.Before(entry.DisabledUntil) {
		if d := entry.DisabledUntil.Sub(now); d > remaining {
			remaining = d
		}
	}

	// Standard cooldown remaining.
	if !entry.CooldownEnd.IsZero() && now.Before(entry.CooldownEnd) {
		if d := entry.CooldownEnd.Sub(now); d > remaining {
			remaining = d
		}
	}

	return remaining
}

// getOrCreate returns the entry for provider, creating one if needed.
// Caller must hold ct.mu.
func (ct *CooldownTracker) getOrCreate(provider string) *cooldownEntry {
	entry := ct.entries[provider]
	if entry == nil {
		entry = &cooldownEntry{
			FailureCounts: make(map[types.FailoverReason]int),
		}
		ct.entries[provider] = entry
	}
	return entry
}

// ErrorCount returns the current error count for a provider.
func (ct *CooldownTracker) ErrorCount(provider string) int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	entry := ct.entries[provider]
	if entry == nil {
		return 0
	}
	return entry.ErrorCount
}

// FailureCount returns the failure count for a specific reason.
func (ct *CooldownTracker) FailureCount(provider string, reason types.FailoverReason) int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	entry := ct.entries[provider]
	if entry == nil {
		return 0
	}
	return entry.FailureCounts[reason]
}

// CalculateCooldown returns the standard exponential backoff duration:
// min(1h, 1min * 5^min(n-1, 3)).
//
//	1 error  → 1 min
//	2 errors → 5 min
//	3 errors → 25 min
//	4+ errors → 1 hour (cap)
func CalculateCooldown(errorCount int) time.Duration {
	if errorCount <= 0 {
		return 0
	}
	exp := min(errorCount-1, 3)
	raw := math.Pow(5, float64(exp))
	if math.IsInf(raw, 0) || math.IsNaN(raw) || raw > float64(time.Hour/time.Minute) {
		return time.Hour
	}
	d := time.Minute * time.Duration(raw)
	return min(d, time.Hour)
}

// calculateBillingCooldown returns the billing-specific exponential backoff:
// min(24h, 5h * 2^min(n-1, 10)).
//
//	1 error  → 5 hours
//	2 errors → 10 hours
//	3 errors → 20 hours
//	4+ errors → 24 hours (cap)
func calculateBillingCooldown(billingErrorCount int) time.Duration {
	const baseMs int64 = 5 * 60 * 60 * 1000 // 5 hours
	const maxMs int64 = 24 * 60 * 60 * 1000 // 24 hours

	n := max(1, billingErrorCount)
	exp := min(n-1, 10)
	raw := float64(baseMs) * math.Pow(2, float64(exp))
	if math.IsInf(raw, 0) || math.IsNaN(raw) || raw > float64(maxMs) {
		return time.Duration(maxMs) * time.Millisecond
	}
	return time.Duration(int64(raw)) * time.Millisecond
}
