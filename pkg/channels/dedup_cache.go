package channels

import (
	"sync"
	"time"
)

const defaultDedupMaxSize = 100000

// DedupCache is a TTL-based message deduplication cache with background cleanup.
type DedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
	maxSize int
	stop    chan struct{}
}

func NewDedupCache(ttl time.Duration) *DedupCache {
	d := &DedupCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		maxSize: defaultDedupMaxSize,
		stop:    make(chan struct{}),
	}
	go d.backgroundCleanup()
	return d
}

// Check returns true if id was already seen (duplicate).
func (d *DedupCache) Check(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.entries[id]; ok {
		return true
	}
	if len(d.entries) >= d.maxSize {
		d.evictOldest()
	}
	d.entries[id] = time.Now()
	return false
}

// Close stops the background cleanup goroutine.
func (d *DedupCache) Close() {
	select {
	case <-d.stop:
	default:
		close(d.stop)
	}
}

func (d *DedupCache) backgroundCleanup() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.mu.Lock()
			d.cleanup()
			d.mu.Unlock()
		}
	}
}

func (d *DedupCache) cleanup() {
	cutoff := time.Now().Add(-d.ttl)
	for k, t := range d.entries {
		if t.Before(cutoff) {
			delete(d.entries, k)
		}
	}
}

func (d *DedupCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, t := range d.entries {
		if oldestKey == "" || t.Before(oldestTime) {
			oldestKey = k
			oldestTime = t
		}
	}
	if oldestKey != "" {
		delete(d.entries, oldestKey)
	}
}
