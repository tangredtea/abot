// Package queue provides lane-based command queue for concurrency control.
//
// Inspired by OpenClaw's command-queue system:
// - Global Lane: All commands execute serially
// - Session Lane: Commands per session execute serially
// - Configurable concurrency per lane
package queue

import (
	"context"
	"sync"
	"time"
)

// Lane represents a command execution lane.
type Lane string

const (
	GlobalLane  Lane = "global"  // All commands serial
	SessionLane Lane = "session" // Per-session serial
)

// Command represents a queued command.
type Command struct {
	ID      string
	Lane    Lane
	Key     string // Session key for SessionLane
	Execute func(context.Context) error
}

// Queue manages lane-based command execution.
type Queue struct {
	globalCh   chan *Command
	sessionChs map[string]chan *Command
	mu         sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewQueue creates a command queue.
func NewQueue(ctx context.Context) *Queue {
	ctx, cancel := context.WithCancel(ctx)
	q := &Queue{
		globalCh:   make(chan *Command, 100),
		sessionChs: make(map[string]chan *Command),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start global lane worker
	q.wg.Add(1)
	go q.runGlobalWorker()

	return q
}

// Enqueue adds a command to the queue.
func (q *Queue) Enqueue(cmd *Command) {
	if cmd.Lane == GlobalLane {
		q.globalCh <- cmd
	} else {
		q.enqueueSession(cmd)
	}
}

// enqueueSession enqueues to session-specific lane.
func (q *Queue) enqueueSession(cmd *Command) {
	q.mu.Lock()
	ch, exists := q.sessionChs[cmd.Key]
	if !exists {
		ch = make(chan *Command, 10)
		q.sessionChs[cmd.Key] = ch
		q.wg.Add(1)
		go q.runSessionWorker(cmd.Key, ch)
	}
	q.mu.Unlock()

	select {
	case ch <- cmd:
	case <-q.ctx.Done():
	}
}

// runGlobalWorker processes global lane commands.
func (q *Queue) runGlobalWorker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.ctx.Done():
			return
		case cmd := <-q.globalCh:
			_ = cmd.Execute(q.ctx)
		}
	}
}

// runSessionWorker processes session lane commands.
// The worker exits after 5 minutes of inactivity, cleaning up its map entry.
func (q *Queue) runSessionWorker(key string, ch chan *Command) {
	defer q.wg.Done()
	defer func() {
		q.mu.Lock()
		delete(q.sessionChs, key)
		q.mu.Unlock()
	}()

	idleTimeout := time.NewTimer(5 * time.Minute)
	defer idleTimeout.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case cmd := <-ch:
			_ = cmd.Execute(q.ctx)
			idleTimeout.Reset(5 * time.Minute)
		case <-idleTimeout.C:
			return
		}
	}
}

// Stop stops the queue and waits for completion.
func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()
}
