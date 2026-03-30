package vectordb

import (
	"container/list"
	"crypto/md5"
	"encoding/hex"
	"sync"
)

type cacheEntry struct {
	key string
	vec []float32
}

// EmbeddingCache provides in-memory caching for embeddings to reduce API costs.
type EmbeddingCache struct {
	cache   map[string]*list.Element
	lru     *list.List
	mu      sync.RWMutex
	maxSize int
}

// NewEmbeddingCache creates a new embedding cache with the specified max size.
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	if maxSize <= 0 {
		maxSize = 10000 // default 10k entries
	}
	return &EmbeddingCache{
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

// Get retrieves a cached embedding for the given text.
func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
	key := c.hash(text)
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		return elem.Value.(*cacheEntry).vec, true
	}
	return nil, false
}

// Set stores an embedding in the cache.
func (c *EmbeddingCache) Set(text string, vec []float32) {
	key := c.hash(text)
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		elem.Value.(*cacheEntry).vec = vec
		return
	}

	if c.lru.Len() >= c.maxSize {
		oldest := c.lru.Back()
		if oldest != nil {
			c.lru.Remove(oldest)
			delete(c.cache, oldest.Value.(*cacheEntry).key)
		}
	}

	entry := &cacheEntry{key: key, vec: vec}
	elem := c.lru.PushFront(entry)
	c.cache[key] = elem
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
	c.cache = make(map[string]*list.Element)
	c.lru = list.New()
}

// Size returns the current number of cached embeddings.
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}
