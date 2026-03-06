package vectordb

import (
	"crypto/md5"
	"encoding/hex"
	"sync"
)

// EmbeddingCache provides in-memory caching for embeddings to reduce API costs.
type EmbeddingCache struct {
	cache   map[string][]float32
	mu      sync.RWMutex
	maxSize int
}

// NewEmbeddingCache creates a new embedding cache with the specified max size.
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	if maxSize <= 0 {
		maxSize = 10000 // default 10k entries
	}
	return &EmbeddingCache{
		cache:   make(map[string][]float32),
		maxSize: maxSize,
	}
}

// Get retrieves a cached embedding for the given text.
func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
	key := c.hash(text)
	c.mu.RLock()
	defer c.mu.RUnlock()
	vec, ok := c.cache[key]
	return vec, ok
}

// Set stores an embedding in the cache.
func (c *EmbeddingCache) Set(text string, vec []float32) {
	key := c.hash(text)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction: remove random entry if full
	if len(c.cache) >= c.maxSize {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}

	c.cache[key] = vec
}

// hash generates a cache key from text.
func (c *EmbeddingCache) hash(text string) string {
	h := md5.Sum([]byte(text))
	return hex.EncodeToString(h[:])
}

// Clear removes all cached embeddings.
func (c *EmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float32)
}

// Size returns the current number of cached embeddings.
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
