// Package lock provides session write lock for concurrency protection.
package lock

import (
	"context"
	"fmt"
	"sync"
	"time"
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

// Unlock releases lock for a session.
func (s *SessionLock) Unlock(sessionKey string) {
	s.mu.Lock()
	lock, exists := s.locks[sessionKey]
	s.mu.Unlock()

	if exists {
		lock.Unlock()
	}
}

// WithLock executes function with session lock held.
func (s *SessionLock) WithLock(sessionKey string, fn func() error) error {
	s.Lock(sessionKey)
	defer s.Unlock(sessionKey)
	return fn()
}
