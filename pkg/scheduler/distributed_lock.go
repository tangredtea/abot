// Package scheduler provides distributed heartbeat with leader election.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DistributedLock provides distributed locking for heartbeat coordination.
type DistributedLock struct {
	db          *gorm.DB
	lockName    string
	instanceID  string
	lockTimeout time.Duration
}

// HeartbeatLock represents a distributed lock record in database.
type HeartbeatLock struct {
	LockName   string    `gorm:"primaryKey;size:64"`
	InstanceID string    `gorm:"size:64;not null"`
	AcquiredAt time.Time `gorm:"not null"`
	ExpiresAt  time.Time `gorm:"not null;index"`
}

// NewDistributedLock creates a distributed lock manager.
func NewDistributedLock(db *gorm.DB, instanceID string) *DistributedLock {
	return &DistributedLock{
		db:          db,
		lockName:    "heartbeat_leader",
		instanceID:  instanceID,
		lockTimeout: 5 * time.Minute, // Lock expires after 5 minutes
	}
}

// TryAcquire attempts to acquire the distributed lock.
// Returns true if lock acquired, false if another instance holds it.
func (d *DistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	now := time.Now()
	expiresAt := now.Add(d.lockTimeout)

	// Try to acquire lock using INSERT ... ON DUPLICATE KEY UPDATE
	result := d.db.WithContext(ctx).Exec(`
		INSERT INTO heartbeat_locks (lock_name, instance_id, acquired_at, expires_at)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			instance_id = IF(expires_at < ?, VALUES(instance_id), instance_id),
			acquired_at = IF(expires_at < ?, VALUES(acquired_at), acquired_at),
			expires_at = IF(expires_at < ?, VALUES(expires_at), expires_at)
	`, d.lockName, d.instanceID, now, expiresAt, now, now, now)

	if result.Error != nil {
		return false, fmt.Errorf("acquire lock: %w", result.Error)
	}

	// Check if we got the lock
	var lock HeartbeatLock
	err := d.db.WithContext(ctx).
		Where("lock_name = ?", d.lockName).
		First(&lock).Error

	if err != nil {
		return false, fmt.Errorf("check lock: %w", err)
	}

	return lock.InstanceID == d.instanceID, nil
}

// Release releases the distributed lock.
func (d *DistributedLock) Release(ctx context.Context) error {
	result := d.db.WithContext(ctx).
		Where("lock_name = ? AND instance_id = ?", d.lockName, d.instanceID).
		Delete(&HeartbeatLock{})

	return result.Error
}

// Renew renews the lock expiration time.
func (d *DistributedLock) Renew(ctx context.Context) error {
	expiresAt := time.Now().Add(d.lockTimeout)

	result := d.db.WithContext(ctx).
		Model(&HeartbeatLock{}).
		Where("lock_name = ? AND instance_id = ?", d.lockName, d.instanceID).
		Update("expires_at", expiresAt)

	if result.Error != nil {
		return fmt.Errorf("renew lock: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("lock not held by this instance")
	}

	return nil
}

// CleanupExpired removes expired locks (for maintenance).
func (d *DistributedLock) CleanupExpired(ctx context.Context) error {
	result := d.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&HeartbeatLock{})

	return result.Error
}

// AutoMigrate creates the heartbeat_locks table.
func AutoMigrateHeartbeatLock(db *gorm.DB) error {
	return db.AutoMigrate(&HeartbeatLock{})
}
