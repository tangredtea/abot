package skills

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"abot/pkg/types"
)

// RegisterBuiltins scans the provided embedded FS for skill directories and
// ensures each is registered in the global SkillRegistryStore with tier=builtin.
// The FS should have skill directories at the root (e.g. "memory/SKILL.md").
// Existing records are left untouched (idempotent on restart).
func RegisterBuiltins(ctx context.Context, store types.SkillRegistryStore, builtinFS fs.FS) error {
	entries, err := fs.ReadDir(builtinFS, ".")
	if err != nil {
		return fmt.Errorf("read embedded skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		if rec, _ := store.Get(ctx, name); rec != nil {
			continue
		}

		data, err := fs.ReadFile(builtinFS, name+"/SKILL.md")
		if err != nil {
			slog.Warn("skills: builtin missing SKILL.md", "name", name)
			continue
		}

		parsedName, desc, always := ParseSkillMetadata(string(data))
		if parsedName == "" {
			parsedName = name
		}

		rec := &types.SkillRecord{
			Name:        parsedName,
			Description: desc,
			Version:     "builtin",
			Tier:        types.SkillTierBuiltin,
			AlwaysLoad:  always,
			Status:      types.StatusPublished,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := store.Put(ctx, rec); err != nil {
			return fmt.Errorf("register builtin skill %s: %w", name, err)
		}
		slog.Info("skills: registered builtin", "name", parsedName)
	}
	return nil
}

// LoadBuiltinContent reads a builtin skill's SKILL.md body from the provided FS.
func LoadBuiltinContent(builtinFS fs.FS, name string) (string, error) {
	data, err := fs.ReadFile(builtinFS, name+"/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("builtin skill %s not found: %w", name, err)
	}
	return StripFrontmatter(string(data)), nil
}
