package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"abot/pkg/types"
)

const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// ResolvedSkill is a skill with its content location resolved.
type ResolvedSkill struct {
	Record    *types.SkillRecord
	LocalPath string // local disk path after lazy pull
	Priority  int    // source priority (lower = higher priority)
}

// SkillsLoader loads skill content from BOS/S3 with local disk caching.
type SkillsLoader struct {
	registryStore    types.SkillRegistryStore
	tenantSkillStore types.TenantSkillStore
	tenantStore      types.TenantStore
	objectStore      types.ObjectStore
	cacheDir         string // local disk cache (e.g. /tmp/abot/skills/)

	// In-memory cache: "name:version" → local path.
	// Protected by mu. Complements the on-disk cache.
	mu       sync.RWMutex
	pathCache map[string]string
}

// NewSkillsLoader creates a SkillsLoader.
func NewSkillsLoader(
	registry types.SkillRegistryStore,
	tenantSkill types.TenantSkillStore,
	tenantStore types.TenantStore,
	obj types.ObjectStore,
	cacheDir string,
) *SkillsLoader {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "abot", "skills")
	}
	return &SkillsLoader{
		registryStore:    registry,
		tenantSkillStore: tenantSkill,
		tenantStore:      tenantStore,
		objectStore:      obj,
		cacheDir:         cacheDir,
		pathCache:        make(map[string]string),
	}
}

// LoadForTenant loads all skills visible to a tenant with 4-level priority:
//   P1: tenant-installed (highest)
//   P2: group defaults (tenant's group)
//   P3: global (tier=global, status=published)
//   P4: builtin (tier=builtin, status=published)
// Same-name skills at higher priority shadow lower ones.
func (sl *SkillsLoader) LoadForTenant(ctx context.Context, tenantID string) ([]*ResolvedSkill, error) {
	seen := make(map[string]bool)
	var result []*ResolvedSkill

	// P1: tenant-installed skills — batch fetch records instead of N+1.
	installed, err := sl.tenantSkillStore.ListInstalled(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list installed skills: %w", err)
	}
	if len(installed) > 0 {
		ids := make([]int64, len(installed))
		for i, ts := range installed {
			ids[i] = ts.SkillID
		}
		records, err := sl.registryStore.GetByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("batch get installed skills: %w", err)
		}
		byID := make(map[int64]*types.SkillRecord, len(records))
		for _, rec := range records {
			byID[rec.ID] = rec
		}
		for _, ts := range installed {
			rec, ok := byID[ts.SkillID]
			if !ok {
				slog.Warn("skills: skip installed, record not found",
					"skill_id", ts.SkillID, "tenant", tenantID)
				continue
			}
			if seen[rec.Name] {
				continue
			}
			seen[rec.Name] = true
			result = append(result, &ResolvedSkill{Record: rec, Priority: 1})
		}
	}

	// P2-P4: single query for all published skills, split by tier in memory.
	allPublished, _ := sl.registryStore.List(ctx, types.SkillListOpts{Status: types.StatusPublished})
	byTier := map[types.SkillTier][]*types.SkillRecord{}
	for _, rec := range allPublished {
		byTier[rec.Tier] = append(byTier[rec.Tier], rec)
	}

	// P2: group defaults
	if tenant, err := sl.tenantStore.Get(ctx, tenantID); err == nil && tenant.GroupID != "" {
		for _, rec := range byTier[types.SkillTierGroup] {
			if seen[rec.Name] {
				continue
			}
			seen[rec.Name] = true
			result = append(result, &ResolvedSkill{Record: rec, Priority: 2})
		}
	}

	// P3: global skills
	for _, rec := range byTier[types.SkillTierGlobal] {
		if seen[rec.Name] {
			continue
		}
		seen[rec.Name] = true
		result = append(result, &ResolvedSkill{Record: rec, Priority: 3})
	}

	// P4: builtin skills
	for _, rec := range byTier[types.SkillTierBuiltin] {
		if seen[rec.Name] {
			continue
		}
		seen[rec.Name] = true
		result = append(result, &ResolvedSkill{Record: rec, Priority: 4})
	}

	return result, nil
}

// InvalidateCache removes a skill from the in-memory path cache.
// Should be called after install_skill succeeds.
func (sl *SkillsLoader) InvalidateCache(name, version string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	delete(sl.pathCache, name+":"+version)
}

// ResolveContent lazy-pulls skill content from BOS/S3 to local disk.
// Three-level cache: memory → disk → object store.
func (sl *SkillsLoader) ResolveContent(ctx context.Context, name, version, objectPath string) (string, error) {
	cacheKey := name + ":" + version

	// Level 1: in-memory cache
	sl.mu.RLock()
	p, ok := sl.pathCache[cacheKey]
	sl.mu.RUnlock()
	if ok {
		if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err == nil {
			return p, nil
		}
		// File gone — invalidate stale cache entry and rebuild.
		sl.mu.Lock()
		delete(sl.pathCache, cacheKey)
		sl.mu.Unlock()
	}

	localDir := filepath.Join(sl.cacheDir, name, version)

	// Level 2: local disk
	skillFile := filepath.Join(localDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil {
		sl.mu.Lock()
		sl.pathCache[cacheKey] = localDir
		sl.mu.Unlock()
		return localDir, nil
	}

	// Level 3: pull from object store
	if objectPath == "" {
		return "", fmt.Errorf("no object path for skill %s:%s", name, version)
	}

	rc, err := sl.objectStore.Get(ctx, objectPath)
	if err != nil {
		return "", fmt.Errorf("fetch from object store: %w", err)
	}
	defer rc.Close()

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	dst, err := os.Create(skillFile)
	if err != nil {
		return "", fmt.Errorf("create local file: %w", err)
	}

	if _, err := io.Copy(dst, rc); err != nil {
		dst.Close()
		return "", fmt.Errorf("write local file: %w", err)
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		return "", fmt.Errorf("sync local file: %w", err)
	}
	dst.Close()

	sl.mu.Lock()
	sl.pathCache[cacheKey] = localDir
	sl.mu.Unlock()

	return localDir, nil
}

// LoadSkillContent reads a skill's SKILL.md body (frontmatter stripped).
func (sl *SkillsLoader) LoadSkillContent(localPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(localPath, "SKILL.md"))
	if err != nil {
		return "", err
	}
	return StripFrontmatter(string(data)), nil
}

// BuildSummary builds an XML summary of all skills visible to a tenant.
func (sl *SkillsLoader) BuildSummary(ctx context.Context, tenantID string) (string, error) {
	resolved, err := sl.LoadForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if len(resolved) == 0 {
		return "", nil
	}

	var lines []string
	lines = append(lines, "<skills>")
	for _, rs := range resolved {
		lines = append(lines, "  <skill>")
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapeXML(rs.Record.Name)))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapeXML(rs.Record.Description)))
		lines = append(lines, fmt.Sprintf("    <tier>%s</tier>", escapeXML(string(rs.Record.Tier))))
		if rs.Record.AlwaysLoad {
			lines = append(lines, "    <always_load>true</always_load>")
		}
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")
	return strings.Join(lines, "\n"), nil
}

// IsAlwaysLoad checks if a skill should always be loaded for a tenant.
// Tenant-level override takes precedence over the record default.
func IsAlwaysLoad(rec *types.SkillRecord, ts *types.TenantSkill) bool {
	if ts != nil && ts.AlwaysLoad != nil {
		return *ts.AlwaysLoad
	}
	return rec.AlwaysLoad
}

// --- frontmatter helpers ---

var frontmatterRe = regexp.MustCompile(`(?s)^---(?:\r\n|\n|\r)(.*?)(?:\r\n|\n|\r)---`)
var StripFrontmatterRe = regexp.MustCompile(`(?s)^---(?:\r\n|\n|\r)(.*?)(?:\r\n|\n|\r)---(?:\r\n|\n|\r)*`)

func extractFrontmatter(content string) string {
	m := frontmatterRe.FindStringSubmatch(content)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func StripFrontmatter(content string) string {
	return StripFrontmatterRe.ReplaceAllString(content, "")
}

// ParseSkillMetadata extracts name, description, always from SKILL.md frontmatter.
func ParseSkillMetadata(content string) (name, description string, always bool) {
	fm := extractFrontmatter(content)
	if fm == "" {
		return "", "", false
	}

	// Try JSON first
	var jm struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Always      bool   `json:"always"`
	}
	if json.Unmarshal([]byte(fm), &jm) == nil && jm.Name != "" {
		return jm.Name, jm.Description, jm.Always
	}

	// Simple YAML fallback
	kv := parseSimpleYAML(fm)
	always = kv["always"] == "true"
	return kv["name"], kv["description"], always
}

func parseSimpleYAML(content string) map[string]string {
	result := make(map[string]string)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}
	return result
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ValidateSkillName checks if a skill name is valid.
func ValidateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("name exceeds %d characters", MaxNameLength)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name must be alphanumeric with hyphens")
	}
	return nil
}
