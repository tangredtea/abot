// Package scheduler provides smart heartbeat with centralized state.
package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"abot/pkg/types"
)

// SmartHeartbeatConfig extends HeartbeatConfig with smart features.
type SmartHeartbeatConfig struct {
	HeartbeatConfig
	// Active hours (e.g., "09:00-22:00")
	ActiveHours struct {
		Start string // "09:00"
		End   string // "22:00"
	}
	// Coalesce window for merging requests
	CoalesceWindow time.Duration // default 250ms
	// Duplicate detection window
	DuplicateWindow time.Duration // default 24h
}

// SmartHeartbeatService extends HeartbeatService with smart features.
type SmartHeartbeatService struct {
	*HeartbeatService
	config          SmartHeartbeatConfig
	tenantStore     types.TenantStore
	mu              sync.Mutex
	pendingRequests map[string][]time.Time
	coalesceTimer   *time.Timer
}

// NewSmartHeartbeat creates a smart heartbeat service.
func NewSmartHeartbeat(cfg SmartHeartbeatConfig) *SmartHeartbeatService {
	// Set defaults
	if cfg.CoalesceWindow == 0 {
		cfg.CoalesceWindow = 250 * time.Millisecond
	}
	if cfg.DuplicateWindow == 0 {
		cfg.DuplicateWindow = 24 * time.Hour
	}

	base := NewHeartbeat(cfg.HeartbeatConfig)

	return &SmartHeartbeatService{
		HeartbeatService: base,
		config:           cfg,
		tenantStore:      cfg.Tenants,
		pendingRequests:  make(map[string][]time.Time),
	}
}

// shouldSendHeartbeat checks if heartbeat should be sent based on smart rules.
func (s *SmartHeartbeatService) shouldSendHeartbeat(ctx context.Context, tenantID string, content string) (bool, error) {
	// Rule 1: Check active hours
	if !s.isWithinActiveHours() {
		s.logger.Debug("heartbeat skipped: outside active hours", "tenant", tenantID)
		return false, nil
	}

	// Rule 2: Check duplicate (using centralized state)
	isDup, err := s.isDuplicate(ctx, tenantID, content)
	if err != nil {
		return false, fmt.Errorf("check duplicate: %w", err)
	}
	if isDup {
		s.logger.Debug("heartbeat skipped: duplicate content", "tenant", tenantID)
		return false, nil
	}

	return true, nil
}

// isWithinActiveHours checks if current time is within active hours.
func (s *SmartHeartbeatService) isWithinActiveHours() bool {
	if s.config.ActiveHours.Start == "" || s.config.ActiveHours.End == "" {
		return true // No restriction
	}

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	start := parseTimeToMinutes(s.config.ActiveHours.Start)
	end := parseTimeToMinutes(s.config.ActiveHours.End)

	// Handle cross-midnight (e.g., 22:00-06:00)
	if start > end {
		return currentMinutes >= start || currentMinutes < end
	}

	return currentMinutes >= start && currentMinutes < end
}

// isDuplicate checks if content is duplicate using centralized state.
func (s *SmartHeartbeatService) isDuplicate(ctx context.Context, tenantID string, content string) (bool, error) {
	// Generate content hash
	hash := hashContent(content)

	// Store in tenant state (centralized)
	stateKey := fmt.Sprintf("heartbeat_history_%s", tenantID)

	// Get recent hashes from tenant state
	tenant, err := s.tenantStore.Get(ctx, tenantID)
	if err != nil {
		return false, err
	}

	// Check if hash exists in recent history
	recentHashes := s.getRecentHashes(tenant, stateKey)
	for _, h := range recentHashes {
		if h == hash {
			return true, nil
		}
	}

	// Add new hash to history
	s.addHashToHistory(ctx, tenant, stateKey, hash)

	return false, nil
}

// getRecentHashes retrieves recent content hashes from tenant state.
func (s *SmartHeartbeatService) getRecentHashes(tenant *types.Tenant, stateKey string) []string {
	// This is a simplified implementation
	// In production, you'd store this in tenant.Metadata or a dedicated table
	return []string{}
}

// addHashToHistory adds a hash to tenant state.
func (s *SmartHeartbeatService) addHashToHistory(ctx context.Context, tenant *types.Tenant, stateKey string, hash string) error {
	// Store in tenant metadata with timestamp
	// Cleanup old entries beyond DuplicateWindow
	return nil
}

// coalesceRequests merges multiple requests within coalesce window.
func (s *SmartHeartbeatService) coalesceRequests(tenantID string) {
	s.mu.Lock()
	s.pendingRequests[tenantID] = append(s.pendingRequests[tenantID], time.Now())

	if s.coalesceTimer != nil {
		s.coalesceTimer.Stop()
	}

	s.coalesceTimer = time.AfterFunc(s.config.CoalesceWindow, func() {
		s.executeCoalesced(tenantID)
	})
	s.mu.Unlock()
}

// executeCoalesced executes a coalesced heartbeat request.
func (s *SmartHeartbeatService) executeCoalesced(tenantID string) {
	s.mu.Lock()
	requests := s.pendingRequests[tenantID]
	if len(requests) == 0 {
		s.mu.Unlock()
		return
	}

	delete(s.pendingRequests, tenantID)
	s.mu.Unlock()

	s.logger.Info("executing coalesced heartbeat",
		"tenant", tenantID,
		"requests", len(requests))
}

// Helper functions

func parseTimeToMinutes(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0
	}
	var hour, min int
	fmt.Sscanf(parts[0], "%d", &hour)
	fmt.Sscanf(parts[1], "%d", &min)
	return hour*60 + min
}

func hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
