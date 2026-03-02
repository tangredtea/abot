package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int // Maximum burst size (0 = same as rate)
}

// tokenBucket implements a token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// newTokenBucket creates a new token bucket.
func newTokenBucket(rate, burst int) *tokenBucket {
	capacity := float64(burst)
	if burst == 0 {
		capacity = float64(rate)
	}
	
	return &tokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: float64(rate) / 60.0, // tokens per second
		lastRefill: time.Now(),
	}
}

// allow checks if a request is allowed and consumes a token if so.
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	
	// Refill tokens based on elapsed time
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	// Check if we have tokens available
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	
	return false
}

// rateLimiter manages token buckets for different keys.
type rateLimiter struct {
	buckets map[string]*tokenBucket
	rate    int
	burst   int
	mu      sync.RWMutex
}

// newRateLimiter creates a new rate limiter.
func newRateLimiter(rate, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}

	// Cleanup old buckets every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

// allow checks if a request for the given key is allowed.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = rl.buckets[key]
		if !exists {
			bucket = newTokenBucket(rl.rate, rl.burst)
			rl.buckets[key] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.allow()
}

// cleanup removes inactive buckets.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, bucket := range rl.buckets {
		bucket.mu.Lock()
		inactive := now.Sub(bucket.lastRefill) > 5*time.Minute
		bucket.mu.Unlock()
		
		if inactive {
			delete(rl.buckets, key)
		}
	}
}

// RateLimit returns a middleware that limits requests per IP using token bucket algorithm.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	rl := newRateLimiter(cfg.RequestsPerMinute, cfg.Burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"Rate limit exceeded"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
