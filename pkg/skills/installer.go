package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"abot/pkg/types"
)

// SkillInstaller manages installing and uninstalling skills from GitHub
// to object storage, adapted for ABot's multi-tenant architecture.
type SkillInstaller struct {
	registryStore types.SkillRegistryStore
	objectStore   types.ObjectStore
	client        *http.Client
}

// AvailableSkill describes a skill available for installation from the community directory.
type AvailableSkill struct {
	Name        string   `json:"name"`
	Repository  string   `json:"repository"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
}

const (
	installerTimeout   = 15 * time.Second
	maxSkillFileSize   = 1 * 1024 * 1024 // 1 MB
	maxSkillsListSize  = 2 * 1024 * 1024 // 2 MB
	communitySkillsURL = "https://raw.githubusercontent.com/abot/abot-skills/main/skills.json"
)

// NewSkillInstaller creates a SkillInstaller.
func NewSkillInstaller(
	registry types.SkillRegistryStore,
	objStore types.ObjectStore,
) *SkillInstaller {
	return &SkillInstaller{
		registryStore: registry,
		objectStore:   objStore,
		client:        &http.Client{Timeout: installerTimeout},
	}
}

// InstallFromGitHub downloads SKILL.md from a GitHub repository, uploads it
// to object storage, and creates a global registry record. repo format: "owner/repo".
func (si *SkillInstaller) InstallFromGitHub(ctx context.Context, repo string) (*types.SkillRecord, error) {
	skillName := filepath.Base(repo)
	if err := ValidateSkillName(skillName); err != nil {
		return nil, fmt.Errorf("invalid skill name %q: %w", skillName, err)
	}

	// Check if already exists.
	if existing, _ := si.registryStore.Get(ctx, skillName); existing != nil {
		return nil, fmt.Errorf("skill %q already exists", skillName)
	}

	// Download SKILL.md from GitHub.
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/SKILL.md", repo)
	body, err := si.fetchURL(ctx, rawURL, maxSkillFileSize)
	if err != nil {
		return nil, fmt.Errorf("fetch skill from GitHub: %w", err)
	}

	// Parse metadata.
	name, desc, always, caps := ParseSkillMetadata(string(body))
	if name == "" {
		name = skillName
	}

	// Upload to object storage.
	objectPath := fmt.Sprintf("skills/%s/github/SKILL.md", skillName)
	if err := si.objectStore.Put(ctx, objectPath, bytes.NewReader(body)); err != nil {
		return nil, fmt.Errorf("upload to object store: %w", err)
	}

	// Create registry record.
	now := time.Now()
	meta := map[string]any{"source": "github", "repository": repo}
	if len(caps) > 0 {
		meta["capabilities"] = caps
	}
	rec := &types.SkillRecord{
		Name:        name,
		Description: desc,
		Version:     "github",
		ObjectPath:  objectPath,
		Tier:        types.SkillTierGlobal,
		AlwaysLoad:  always,
		Status:      types.StatusPublished,
		Metadata:    meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := si.registryStore.Put(ctx, rec); err != nil {
		return nil, fmt.Errorf("register skill: %w", err)
	}

	return rec, nil
}

// Uninstall removes a skill from the registry and object storage.
func (si *SkillInstaller) Uninstall(ctx context.Context, skillName string) error {
	rec, err := si.registryStore.Get(ctx, skillName)
	if err != nil {
		return fmt.Errorf("skill %q not found: %w", skillName, err)
	}

	// Clean up object storage.
	if rec.ObjectPath != "" {
		_ = si.objectStore.Delete(ctx, rec.ObjectPath)
	}

	// Delete from registry.
	if err := si.registryStore.Delete(ctx, skillName); err != nil {
		return fmt.Errorf("delete skill record: %w", err)
	}

	return nil
}

// ListAvailableSkills fetches the list of installable skills from the community directory.
func (si *SkillInstaller) ListAvailableSkills(ctx context.Context) ([]AvailableSkill, error) {
	body, err := si.fetchURL(ctx, communitySkillsURL, maxSkillsListSize)
	if err != nil {
		return nil, fmt.Errorf("fetch skills list: %w", err)
	}

	var skills []AvailableSkill
	if err := json.Unmarshal(body, &skills); err != nil {
		return nil, fmt.Errorf("parse skills list: %w", err)
	}
	return skills, nil
}

// --- internal helpers ---

// fetchURL performs a GET request and returns the response body, limited to maxSize bytes.
func (si *SkillInstaller) fetchURL(ctx context.Context, url string, maxSize int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := si.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skills: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}
