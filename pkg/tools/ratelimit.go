package tools

import (
	"fmt"
	"sync"
	"time"
)

// TenantRateLimiter implements a per-tenant token bucket rate limiter.
// Each tenant gets an independent bucket. No Redis dependency — pure in-memory.
// Nil TenantRateLimiter means no rate limiting (backward compatible).
type TenantRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   int     // max burst size
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewTenantRateLimiter creates a rate limiter.
// rate is tokens per second, burst is the max tokens that can accumulate.
// Example: rate=1.0, burst=10 means 60 calls/min with burst of 10.
func NewTenantRateLimiter(rate float64, burst int) *TenantRateLimiter {
	return &TenantRateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow checks whether the tenant is within rate limits.
// Returns nil if allowed, error if rate limit exceeded.
func (rl *TenantRateLimiter) Allow(tenantID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[tenantID]
	if !ok {
		b = &bucket{tokens: float64(rl.burst), lastCheck: now}
		rl.buckets[tenantID] = b
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastCheck = now

	if b.tokens < 1.0 {
		return fmt.Errorf("rate limit exceeded for tenant %q", tenantID)
	}
	b.tokens--
	return nil
}
