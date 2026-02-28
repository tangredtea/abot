package skills

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchCache provides a lightweight search result cache.
// Uses trigram similarity to match similar queries, avoiding redundant remote API calls.
// Thread-safe for concurrent access.
type SearchCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	order      []string // LRU order: oldest first
	maxEntries int
	ttl        time.Duration
}

type cacheEntry struct {
	query     string
	trigrams  []uint32
	results   []SearchResult
	createdAt time.Time
}

// similarityThreshold is the minimum trigram Jaccard similarity to trigger a cache hit.
const similarityThreshold = 0.7

// NewSearchCache creates a search cache.
// maxEntries is the maximum number of entries (LRU eviction when exceeded), ttl is the entry expiry.
func NewSearchCache(maxEntries int, ttl time.Duration) *SearchCache {
	if maxEntries <= 0 {
		maxEntries = 50
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &SearchCache{
		entries:    make(map[string]*cacheEntry),
		order:      make([]string, 0),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

// Get looks up cached results for a query. Exact match takes priority,
// followed by trigram similarity matching. Returns a copy of results and true
// on hit, or nil and false on miss.
func (sc *SearchCache) Get(query string) ([]SearchResult, bool) {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return nil, false
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Exact match.
	if entry, ok := sc.entries[normalized]; ok {
		if time.Since(entry.createdAt) < sc.ttl {
			sc.moveToEndLocked(normalized)
			return copyResults(entry.results), true
		}
	}

	// Similarity match.
	queryTrigrams := BuildTrigrams(normalized)
	var bestEntry *cacheEntry
	var bestSim float64

	for _, entry := range sc.entries {
		if time.Since(entry.createdAt) >= sc.ttl {
			continue
		}
		sim := JaccardSimilarity(queryTrigrams, entry.trigrams)
		if sim > bestSim {
			bestSim = sim
			bestEntry = entry
		}
	}

	if bestSim >= similarityThreshold && bestEntry != nil {
		sc.moveToEndLocked(bestEntry.query)
		return copyResults(bestEntry.results), true
	}

	return nil, false
}

// Put stores query results. Updates if already present; evicts LRU entry when at capacity.
func (sc *SearchCache) Put(query string, results []SearchResult) {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Evict expired entries first.
	sc.evictExpiredLocked()

	// Update if already exists.
	if _, ok := sc.entries[normalized]; ok {
		sc.entries[normalized] = &cacheEntry{
			query:     normalized,
			trigrams:  BuildTrigrams(normalized),
			results:   copyResults(results),
			createdAt: time.Now(),
		}
		sc.moveToEndLocked(normalized)
		return
	}

	// Evict LRU when at capacity.
	for len(sc.entries) >= sc.maxEntries && len(sc.order) > 0 {
		oldest := sc.order[0]
		sc.order = sc.order[1:]
		delete(sc.entries, oldest)
	}

	// Insert new entry.
	sc.entries[normalized] = &cacheEntry{
		query:     normalized,
		trigrams:  BuildTrigrams(normalized),
		results:   copyResults(results),
		createdAt: time.Now(),
	}
	sc.order = append(sc.order, normalized)
}

// Len returns the number of cache entries (for testing).
func (sc *SearchCache) Len() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.entries)
}

// --- internal methods ---

func (sc *SearchCache) evictExpiredLocked() {
	now := time.Now()
	newOrder := make([]string, 0, len(sc.order))
	for _, key := range sc.order {
		entry, ok := sc.entries[key]
		if !ok || now.Sub(entry.createdAt) >= sc.ttl {
			delete(sc.entries, key)
			continue
		}
		newOrder = append(newOrder, key)
	}
	sc.order = newOrder
}

func (sc *SearchCache) moveToEndLocked(key string) {
	for i, k := range sc.order {
		if k == key {
			sc.order = append(sc.order[:i], sc.order[i+1:]...)
			break
		}
	}
	sc.order = append(sc.order, key)
}

// normalizeQuery lowercases and trims whitespace from a query string.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// BuildTrigrams generates a sorted, deduplicated set of trigram hashes from a string.
// For example, "hello" produces hashes of {"hel", "ell", "llo"}.
func BuildTrigrams(s string) []uint32 {
	if len(s) < 3 {
		return nil
	}

	trigrams := make([]uint32, 0, len(s)-2)
	for i := 0; i <= len(s)-3; i++ {
		trigrams = append(trigrams, uint32(s[i])<<16|uint32(s[i+1])<<8|uint32(s[i+2]))
	}

	// Sort and deduplicate.
	sort.Slice(trigrams, func(i, j int) bool { return trigrams[i] < trigrams[j] })
	n := 1
	for i := 1; i < len(trigrams); i++ {
		if trigrams[i] != trigrams[i-1] {
			trigrams[n] = trigrams[i]
			n++
		}
	}
	return trigrams[:n]
}

// JaccardSimilarity computes the Jaccard similarity |A∩B| / |A∪B| of two sorted trigram sets.
func JaccardSimilarity(a, b []uint32) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	i, j := 0, 0
	intersection := 0

	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			intersection++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}

	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

// copyResults returns a shallow copy of search results to isolate internal state.
func copyResults(results []SearchResult) []SearchResult {
	if results == nil {
		return nil
	}
	cp := make([]SearchResult, len(results))
	copy(cp, results)
	return cp
}
