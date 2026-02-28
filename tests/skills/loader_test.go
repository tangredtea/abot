package skills_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"abot/pkg/skills"
	"abot/pkg/types"
)

// --- mock stores ---

type mockSkillRegistryStore struct {
	records map[string]*types.SkillRecord
	byID    map[int64]*types.SkillRecord
}

func newMockRegistry() *mockSkillRegistryStore {
	return &mockSkillRegistryStore{
		records: make(map[string]*types.SkillRecord),
		byID:    make(map[int64]*types.SkillRecord),
	}
}

func (m *mockSkillRegistryStore) Get(_ context.Context, name string) (*types.SkillRecord, error) {
	r, ok := m.records[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return r, nil
}

func (m *mockSkillRegistryStore) GetByID(_ context.Context, id int64) (*types.SkillRecord, error) {
	r, ok := m.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return r, nil
}

func (m *mockSkillRegistryStore) List(_ context.Context, opts types.SkillListOpts) ([]*types.SkillRecord, error) {
	var result []*types.SkillRecord
	for _, r := range m.records {
		if opts.Tier != "" && r.Tier != opts.Tier {
			continue
		}
		if opts.Status != "" && r.Status != opts.Status {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

func (m *mockSkillRegistryStore) Put(_ context.Context, rec *types.SkillRecord) error {
	m.records[rec.Name] = rec
	if rec.ID > 0 {
		m.byID[rec.ID] = rec
	}
	return nil
}

func (m *mockSkillRegistryStore) GetByIDs(_ context.Context, ids []int64) ([]*types.SkillRecord, error) {
	var out []*types.SkillRecord
	for _, id := range ids {
		if r, ok := m.byID[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *mockSkillRegistryStore) Delete(_ context.Context, name string) error {
	delete(m.records, name)
	return nil
}

func (m *mockSkillRegistryStore) add(rec *types.SkillRecord) {
	m.records[rec.Name] = rec
	m.byID[rec.ID] = rec
}

type mockTenantSkillStore struct {
	installed map[string][]*types.TenantSkill // tenantID -> list
}

func newMockTenantSkillStore() *mockTenantSkillStore {
	return &mockTenantSkillStore{installed: make(map[string][]*types.TenantSkill)}
}

func (m *mockTenantSkillStore) Install(_ context.Context, ts *types.TenantSkill) error {
	m.installed[ts.TenantID] = append(m.installed[ts.TenantID], ts)
	return nil
}

func (m *mockTenantSkillStore) Uninstall(_ context.Context, tenantID string, skillID int64) error {
	return nil
}

func (m *mockTenantSkillStore) ListInstalled(_ context.Context, tenantID string) ([]*types.TenantSkill, error) {
	return m.installed[tenantID], nil
}

func (m *mockTenantSkillStore) UpdateConfig(_ context.Context, _ string, _ int64, _ map[string]any) error {
	return nil
}

type mockTenantStore struct {
	tenants map[string]*types.Tenant
}

func newMockTenantStore() *mockTenantStore {
	return &mockTenantStore{tenants: make(map[string]*types.Tenant)}
}

func (m *mockTenantStore) Get(_ context.Context, id string) (*types.Tenant, error) {
	t, ok := m.tenants[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}

func (m *mockTenantStore) Put(_ context.Context, t *types.Tenant) error {
	m.tenants[t.TenantID] = t
	return nil
}

func (m *mockTenantStore) List(_ context.Context, _ string) ([]*types.Tenant, error) {
	return nil, nil
}

func (m *mockTenantStore) Delete(_ context.Context, _ string) error {
	return nil
}

type mockObjectStore struct {
	data map[string][]byte
}

func newMockObjectStore() *mockObjectStore {
	return &mockObjectStore{data: make(map[string][]byte)}
}

func (m *mockObjectStore) Put(_ context.Context, path string, r io.Reader) error {
	d, _ := io.ReadAll(r)
	m.data[path] = d
	return nil
}

func (m *mockObjectStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	d, ok := m.data[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return io.NopCloser(strings.NewReader(string(d))), nil
}

func (m *mockObjectStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockObjectStore) Exists(_ context.Context, path string) (bool, error) {
	_, ok := m.data[path]
	return ok, nil
}

// --- tests ---

func TestLoadForTenant_PriorityOrder(t *testing.T) {
	reg := newMockRegistry()
	tss := newMockTenantSkillStore()
	ts := newMockTenantStore()
	obj := newMockObjectStore()

	// Add skills at different tiers with same name "overlap"
	reg.add(&types.SkillRecord{ID: 1, Name: "overlap", Tier: types.SkillTierBuiltin, Status: "published"})
	reg.add(&types.SkillRecord{ID: 2, Name: "global-only", Tier: types.SkillTierGlobal, Status: "published"})
	reg.add(&types.SkillRecord{ID: 3, Name: "builtin-only", Tier: types.SkillTierBuiltin, Status: "published"})

	// Tenant installs "overlap" — should shadow the builtin
	tss.installed["t1"] = []*types.TenantSkill{
		{TenantID: "t1", SkillID: 1, InstalledAt: time.Now()},
	}

	ts.tenants["t1"] = &types.Tenant{TenantID: "t1"}

	loader := skills.NewSkillsLoader(reg, tss, ts, obj, t.TempDir())
	resolved, err := loader.LoadForTenant(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}

	// Should have 3 skills: overlap(P1), global-only(P3), builtin-only(P4)
	if len(resolved) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(resolved))
	}

	// First should be tenant-installed (priority 1)
	if resolved[0].Priority != 1 || resolved[0].Record.Name != "overlap" {
		t.Errorf("expected P1 overlap, got P%d %s", resolved[0].Priority, resolved[0].Record.Name)
	}
}

func TestResolveContent_ThreeLevelCache(t *testing.T) {
	reg := newMockRegistry()
	tss := newMockTenantSkillStore()
	ts := newMockTenantStore()
	obj := newMockObjectStore()

	skillContent := "---\nname: test\n---\n# Test Skill\nHello"
	obj.data["skills/test/v1.tar.gz"] = []byte(skillContent)

	cacheDir := t.TempDir()
	loader := skills.NewSkillsLoader(reg, tss, ts, obj, cacheDir)

	// First call: pulls from object store
	path1, err := loader.ResolveContent(context.Background(), "test", "v1", "skills/test/v1.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if path1 == "" {
		t.Fatal("expected non-empty path")
	}

	// Verify file exists on disk
	skillFile := filepath.Join(path1, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != skillContent {
		t.Errorf("content mismatch: got %q", string(data))
	}

	// Second call: should hit memory cache
	path2, err := loader.ResolveContent(context.Background(), "test", "v1", "skills/test/v1.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if path1 != path2 {
		t.Error("expected same path from cache")
	}
}

func TestParseSkillMetadata_YAML(t *testing.T) {
	content := "---\nname: memory\ndescription: Persistent memory skill\nalways: true\n---\n# Memory\nBody here"
	name, desc, always := skills.ParseSkillMetadata(content)
	if name != "memory" {
		t.Errorf("name: got %q, want %q", name, "memory")
	}
	if desc != "Persistent memory skill" {
		t.Errorf("description: got %q", desc)
	}
	if !always {
		t.Error("expected always=true")
	}
}

func TestParseSkillMetadata_JSON(t *testing.T) {
	content := "---\n{\"name\":\"search\",\"description\":\"Web search\",\"always\":false}\n---\n# Search"
	name, desc, always := skills.ParseSkillMetadata(content)
	if name != "search" {
		t.Errorf("name: got %q, want %q", name, "search")
	}
	if desc != "Web search" {
		t.Errorf("description: got %q", desc)
	}
	if always {
		t.Error("expected always=false")
	}
}

func TestParseSkillMetadata_NoFrontmatter(t *testing.T) {
	name, desc, always := skills.ParseSkillMetadata("# Just a heading\nNo frontmatter here")
	if name != "" || desc != "" || always {
		t.Errorf("expected empty metadata, got name=%q desc=%q always=%v", name, desc, always)
	}
}

func TestStripFrontmatter(t *testing.T) {
	input := "---\nname: test\n---\n# Body\nContent"
	got := skills.StripFrontmatter(input)
	if got != "# Body\nContent" {
		t.Errorf("StripFrontmatter: got %q", got)
	}
}

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"memory", false},
		{"web-search", false},
		{"my-cool-skill", false},
		{"", true},
		{"-bad", true},
		{"bad-", true},
		{"has space", true},
		{"has_underscore", true},
		{strings.Repeat("a", skills.MaxNameLength+1), true},
	}
	for _, tt := range tests {
		err := skills.ValidateSkillName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSkillName(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestLoadSkillContent_StripsHeader(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: test\n---\n# Skill Body\nHello world"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := skills.NewSkillsLoader(newMockRegistry(), newMockTenantSkillStore(), newMockTenantStore(), newMockObjectStore(), t.TempDir())
	body, err := loader.LoadSkillContent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if body != "# Skill Body\nHello world" {
		t.Errorf("got %q", body)
	}
}

func TestBuildSummary_XMLFormat(t *testing.T) {
	reg := newMockRegistry()
	tss := newMockTenantSkillStore()
	ts := newMockTenantStore()
	obj := newMockObjectStore()

	reg.add(&types.SkillRecord{ID: 1, Name: "memory", Tier: types.SkillTierBuiltin, Status: "published", Description: "Save & recall"})
	reg.add(&types.SkillRecord{ID: 2, Name: "search", Tier: types.SkillTierGlobal, Status: "published", Description: "Web search"})

	ts.tenants["t1"] = &types.Tenant{TenantID: "t1"}
	loader := skills.NewSkillsLoader(reg, tss, ts, obj, t.TempDir())

	summary, err := loader.BuildSummary(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "<skills>") {
		t.Error("missing <skills> tag")
	}
	if !strings.Contains(summary, "<name>memory</name>") {
		t.Error("missing memory skill")
	}
	if !strings.Contains(summary, "<name>search</name>") {
		t.Error("missing search skill")
	}
	if !strings.Contains(summary, "Save &amp; recall") {
		t.Error("XML escaping not applied")
	}
}

func TestIsAlwaysLoad_TenantOverride(t *testing.T) {
	rec := &types.SkillRecord{AlwaysLoad: true}

	// No tenant override — use record default
	if !skills.IsAlwaysLoad(rec, nil) {
		t.Error("expected true from record default")
	}

	// Tenant override to false
	f := false
	ts := &types.TenantSkill{AlwaysLoad: &f}
	if skills.IsAlwaysLoad(rec, ts) {
		t.Error("expected false from tenant override")
	}

	// Tenant override to true on a record that defaults false
	rec2 := &types.SkillRecord{AlwaysLoad: false}
	tr := true
	ts2 := &types.TenantSkill{AlwaysLoad: &tr}
	if !skills.IsAlwaysLoad(rec2, ts2) {
		t.Error("expected true from tenant override")
	}
}
