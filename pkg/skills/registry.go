package skills

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"abot/pkg/types"
)

const defaultMaxConcurrentSearches = 2

// SearchResult represents a single result from a skill registry search.
type SearchResult struct {
	Score        float64 `json:"score"`
	Slug         string  `json:"slug"`
	DisplayName  string  `json:"display_name"`
	Summary      string  `json:"summary"`
	Version      string  `json:"version"`
	RegistryName string  `json:"registry_name"`
}

// SkillMeta holds metadata about a skill from a registry.
type SkillMeta struct {
	Slug             string `json:"slug"`
	DisplayName      string `json:"display_name"`
	Summary          string `json:"summary"`
	LatestVersion    string `json:"latest_version"`
	IsMalwareBlocked bool   `json:"is_malware_blocked"`
	IsSuspicious     bool   `json:"is_suspicious"`
	RegistryName     string `json:"registry_name"`
}

// InstallResult is returned by DownloadAndInstall.
type InstallResult struct {
	Version          string
	IsMalwareBlocked bool
	IsSuspicious     bool
	Summary          string
}

// SkillRegistry is the interface that all remote skill registries must implement.
type SkillRegistry interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	GetSkillMeta(ctx context.Context, slug string) (*SkillMeta, error)
	// DownloadAndInstall downloads the skill package and uploads it to the object store
	// at objectPath. Returns metadata for moderation decisions.
	DownloadAndInstall(ctx context.Context, slug, version string, objStore types.ObjectStore, objectPath string) (*InstallResult, error)
}

// RegistryConfig holds configuration for all skill registries.
type RegistryConfig struct {
	ClawHub               ClawHubConfig
	MaxConcurrentSearches int
}

// ClawHubConfig configures the ClawHub registry.
type ClawHubConfig struct {
	Enabled         bool
	BaseURL         string
	AuthToken       string
	SearchPath      string
	SkillsPath      string
	DownloadPath    string
	Timeout         int // seconds
	MaxZipSize      int // bytes
	MaxResponseSize int // bytes
}

// RegistryManager coordinates multiple skill registries.
type RegistryManager struct {
	registries    []SkillRegistry
	maxConcurrent int
	mu            sync.RWMutex
}

// NewRegistryManager creates an empty RegistryManager.
func NewRegistryManager() *RegistryManager {
	return &RegistryManager{
		registries:    make([]SkillRegistry, 0),
		maxConcurrent: defaultMaxConcurrentSearches,
	}
}

// NewRegistryManagerFromConfig builds a RegistryManager from config.
func NewRegistryManagerFromConfig(cfg RegistryConfig) *RegistryManager {
	rm := NewRegistryManager()
	if cfg.MaxConcurrentSearches > 0 {
		rm.maxConcurrent = cfg.MaxConcurrentSearches
	}
	return rm
}

// AddRegistry adds a registry to the manager.
func (rm *RegistryManager) AddRegistry(r SkillRegistry) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.registries = append(rm.registries, r)
}

// GetRegistry returns a registry by name, or nil if not found.
func (rm *RegistryManager) GetRegistry(name string) SkillRegistry {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for _, r := range rm.registries {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

// SearchAll fans out the query to all registries concurrently
// and merges results sorted by score descending.
func (rm *RegistryManager) SearchAll(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	rm.mu.RLock()
	regs := make([]SkillRegistry, len(rm.registries))
	copy(regs, rm.registries)
	rm.mu.RUnlock()

	if len(regs) == 0 {
		return nil, fmt.Errorf("no registries configured")
	}

	type regResult struct {
		results []SearchResult
		err     error
	}

	sem := make(chan struct{}, rm.maxConcurrent)
	resultsCh := make(chan regResult, len(regs))

	var wg sync.WaitGroup
	for _, reg := range regs {
		wg.Add(1)
		go func(r SkillRegistry) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				resultsCh <- regResult{err: ctx.Err()}
				return
			}

			searchCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			defer cancel()

			results, err := r.Search(searchCtx, query, limit)
			if err != nil {
				slog.Warn("skills: registry search failed", "registry", r.Name(), "err", err)
				resultsCh <- regResult{err: err}
				return
			}
			resultsCh <- regResult{results: results}
		}(reg)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var merged []SearchResult
	var lastErr error
	var anyOK bool

	for rr := range resultsCh {
		if rr.err != nil {
			lastErr = rr.err
			continue
		}
		anyOK = true
		merged = append(merged, rr.results...)
	}

	if !anyOK && lastErr != nil {
		return nil, fmt.Errorf("all registries failed: %w", lastErr)
	}

	SortByScoreDesc(merged)

	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}

	return merged, nil
}

// SortByScoreDesc sorts SearchResults by Score descending (insertion sort for small slices).
func SortByScoreDesc(results []SearchResult) {
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].Score < key.Score {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}
