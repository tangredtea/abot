package cache

import (
	"container/list"
	"sync"
)

// lruCache is a generic thread-safe LRU cache.
type lruCache[K comparable, V any] struct {
	mu       sync.RWMutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

func NewLRU[K comparable, V any](capacity int) *lruCache[K, V] {
	return &lruCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}
}

func (c *lruCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	el, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	c.mu.Lock()
	c.order.MoveToFront(el)
	c.mu.Unlock()
	return el.Value.(*entry[K, V]).value, true
}

func (c *lruCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*entry[K, V]).value = value
		return
	}
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*entry[K, V]).key)
		}
	}
	el := c.order.PushFront(&entry[K, V]{key: key, value: value})
	c.items[key] = el
}

func (c *lruCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.order.Remove(el)
		delete(c.items, key)
	}
}
