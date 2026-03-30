// Package lock provides session write lock for concurrency protection.
package lock

import (
	"sync"
)

// SessionLock provides per-session write locking.
type SessionLock struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// NewSessionLock creates a session lock manager.
func NewSessionLock() *SessionLock {
	return &SessionLock{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock acquires lock for a session.
func (s *SessionLock) Lock(sessionKey string) {
	s.mu.Lock()
	lock, exists := s.locks[sessionKey]
	if !exists {
		lock = &sync.Mutex{}
		s.locks[sessionKey] = lock
	}
	s.mu.Unlock()

	lock.Lock()
}

// Unlock releases lock for a session and removes it from the map if not contended.
func (s *SessionLock) Unlock(sessionKey string) {
	s.mu.Lock()
	lock, exists := s.locks[sessionKey]
	s.mu.Unlock()

	if exists {
		lock.Unlock()
	}
}

// Cleanup removes stale lock entries that are not currently held.
// Should be called periodically (e.g., every 10 minutes).
func (s *SessionLock) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, lock := range s.locks {
		if lock.TryLock() {
			lock.Unlock()
			delete(s.locks, key)
			removed++
		}
	}
	return removed
}

// WithLock executes function with session lock held.
func (s *SessionLock) WithLock(sessionKey string, fn func() error) error {
	s.Lock(sessionKey)
	defer s.Unlock(sessionKey)
	return fn()
}
