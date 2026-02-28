package skills_test

import (
	"context"
	"fmt"
	"testing"

	"abot/pkg/skills"
	"abot/pkg/types"
)

// --- mock registry ---

type mockRemoteRegistry struct {
	name    string
	results []skills.SearchResult
	err     error
}

func (m *mockRemoteRegistry) Name() string { return m.name }

func (m *mockRemoteRegistry) Search(_ context.Context, _ string, _ int) ([]skills.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockRemoteRegistry) GetSkillMeta(_ context.Context, _ string) (*skills.SkillMeta, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRemoteRegistry) DownloadAndInstall(_ context.Context, _, _ string, _ types.ObjectStore, _ string) (*skills.InstallResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// --- tests ---

func TestRegistryManager_SearchAll_MergesAndSorts(t *testing.T) {
	rm := skills.NewRegistryManager()
	rm.AddRegistry(&mockRemoteRegistry{
		name: "alpha",
		results: []skills.SearchResult{
			{Slug: "a1", Score: 0.8, RegistryName: "alpha"},
			{Slug: "a2", Score: 0.5, RegistryName: "alpha"},
		},
	})
	rm.AddRegistry(&mockRemoteRegistry{
		name: "beta",
		results: []skills.SearchResult{
			{Slug: "b1", Score: 0.9, RegistryName: "beta"},
			{Slug: "b2", Score: 0.3, RegistryName: "beta"},
		},
	})

	results, err := rm.SearchAll(context.Background(), "test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Should be sorted by score descending
	if results[0].Slug != "b1" || results[0].Score != 0.9 {
		t.Errorf("first result: got %s (%.1f), want b1 (0.9)", results[0].Slug, results[0].Score)
	}
	if results[3].Slug != "b2" || results[3].Score != 0.3 {
		t.Errorf("last result: got %s (%.1f), want b2 (0.3)", results[3].Slug, results[3].Score)
	}
}

func TestRegistryManager_SearchAll_WithLimit(t *testing.T) {
	rm := skills.NewRegistryManager()
	rm.AddRegistry(&mockRemoteRegistry{
		name: "r1",
		results: []skills.SearchResult{
			{Slug: "s1", Score: 0.9},
			{Slug: "s2", Score: 0.8},
			{Slug: "s3", Score: 0.7},
		},
	})

	results, err := rm.SearchAll(context.Background(), "test", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Slug != "s1" {
		t.Errorf("expected s1 first, got %s", results[0].Slug)
	}
}

func TestRegistryManager_SearchAll_NoRegistries(t *testing.T) {
	rm := skills.NewRegistryManager()
	_, err := rm.SearchAll(context.Background(), "test", 10)
	if err == nil {
		t.Error("expected error with no registries")
	}
}

func TestRegistryManager_SearchAll_PartialFailure(t *testing.T) {
	rm := skills.NewRegistryManager()
	rm.AddRegistry(&mockRemoteRegistry{
		name: "good",
		results: []skills.SearchResult{
			{Slug: "ok", Score: 1.0},
		},
	})
	rm.AddRegistry(&mockRemoteRegistry{
		name: "bad",
		err:  fmt.Errorf("network error"),
	})

	results, err := rm.SearchAll(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("partial failure should not error: %v", err)
	}
	if len(results) != 1 || results[0].Slug != "ok" {
		t.Errorf("expected 1 result from good registry, got %d", len(results))
	}
}

func TestRegistryManager_GetRegistry(t *testing.T) {
	rm := skills.NewRegistryManager()
	rm.AddRegistry(&mockRemoteRegistry{name: "clawhub"})

	if r := rm.GetRegistry("clawhub"); r == nil {
		t.Error("expected to find clawhub")
	}
	if r := rm.GetRegistry("nonexistent"); r != nil {
		t.Error("expected nil for nonexistent registry")
	}
}

func TestSortByScoreDesc(t *testing.T) {
	results := []skills.SearchResult{
		{Slug: "low", Score: 0.1},
		{Slug: "high", Score: 0.9},
		{Slug: "mid", Score: 0.5},
	}
	skills.SortByScoreDesc(results)
	if results[0].Slug != "high" || results[1].Slug != "mid" || results[2].Slug != "low" {
		t.Errorf("sort order wrong: %v", results)
	}
}
