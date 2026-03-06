// Distributed heartbeat with leader election
package scheduler

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DistributedHeartbeatConfig extends HeartbeatConfig with distributed lock.
type DistributedHeartbeatConfig struct {
	HeartbeatConfig
	DB         *gorm.DB
	InstanceID string // Unique instance identifier
}

// DistributedHeartbeatService wraps HeartbeatService with leader election.
type DistributedHeartbeatService struct {
	*HeartbeatService
	lock       *DistributedLock
	instanceID string
	isLeader   bool
}

// NewDistributedHeartbeat creates a heartbeat service with leader election.
func NewDistributedHeartbeat(cfg DistributedHeartbeatConfig) *DistributedHeartbeatService {
	base := NewHeartbeat(cfg.HeartbeatConfig)
	lock := NewDistributedLock(cfg.DB, cfg.InstanceID)

	return &DistributedHeartbeatService{
		HeartbeatService: base,
		lock:             lock,
		instanceID:       cfg.InstanceID,
	}
}

// Start begins the heartbeat loop with leader election.
func (d *DistributedHeartbeatService) Start(ctx context.Context) error {
	d.mu.Lock()
	ctx, d.cancel = context.WithCancel(ctx)
	d.mu.Unlock()

	go d.leaderLoop(ctx)
	d.logger.Info("distributed heartbeat started", "instance", d.instanceID)
	return nil
}

// leaderLoop continuously tries to acquire leadership and run heartbeat.
func (d *DistributedHeartbeatService) leaderLoop(ctx context.Context) {
	defer close(d.done)

	ticker := time.NewTicker(30 * time.Second) // Try acquire every 30s
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.releaseLock(context.Background())
			return

		case <-ticker.C:
			acquired, err := d.lock.TryAcquire(ctx)
			if err != nil {
				d.logger.Error("failed to acquire lock", "error", err)
				continue
			}

			if acquired {
				if !d.isLeader {
					d.logger.Info("became heartbeat leader", "instance", d.instanceID)
					d.isLeader = true
					go d.runAsLeader(ctx)
				}
			} else {
				if d.isLeader {
					d.logger.Info("lost heartbeat leadership", "instance", d.instanceID)
					d.isLeader = false
				}
			}
		}
	}
}

// runAsLeader runs the heartbeat loop while holding leadership.
func (d *DistributedHeartbeatService) runAsLeader(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	renewTicker := time.NewTicker(2 * time.Minute) // Renew lock every 2 minutes
	defer renewTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if !d.isLeader {
				return // Lost leadership
			}
			d.tick(ctx)

		case <-renewTicker.C:
			if err := d.lock.Renew(ctx); err != nil {
				d.logger.Error("failed to renew lock", "error", err)
				d.isLeader = false
				return
			}
		}
	}
}

// releaseLock releases the distributed lock.
func (d *DistributedHeartbeatService) releaseLock(ctx context.Context) {
	if d.isLeader {
		if err := d.lock.Release(ctx); err != nil {
			d.logger.Error("failed to release lock", "error", err)
		}
		d.isLeader = false
	}
}
