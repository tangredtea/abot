package workspace_test

import (
	"context"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/session"

	"abot/pkg/skills"
	"abot/pkg/types"
	"abot/pkg/workspace"
)

// --- mock ReadonlyState ---

type mockState struct {
	data map[string]any
}

func (m *mockState) Get(key string) (any, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return v, nil
}

func (m *mockState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range m.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

// --- mock ReadonlyContext ---

type mockReadonlyContext struct {
	context.Context
	state *mockState
}

func newMockReadonlyContext(kv map[string]any) *mockReadonlyContext {
	return &mockReadonlyContext{
		Context: context.Background(),
		state:   &mockState{data: kv},
	}
}

func (m *mockReadonlyContext) UserContent() *genai.Content          { return nil }
func (m *mockReadonlyContext) InvocationID() string                 { return "inv-1" }
func (m *mockReadonlyContext) AgentName() string                    { return "test-agent" }
func (m *mockReadonlyContext) ReadonlyState() session.ReadonlyState { return m.state }
func (m *mockReadonlyContext) UserID() string                       { return "" }
func (m *mockReadonlyContext) AppName() string                      { return "test" }
func (m *mockReadonlyContext) SessionID() string                    { return "sess-1" }
func (m *mockReadonlyContext) Branch() string                       { return "" }

// --- mock WorkspaceStore ---

type mockWorkspaceStore struct {
	docs map[string]map[string]*types.WorkspaceDoc // tenantID -> docType -> doc
}

func newMockWorkspaceStore() *mockWorkspaceStore {
	return &mockWorkspaceStore{docs: make(map[string]map[string]*types.WorkspaceDoc)}
}

func (m *mockWorkspaceStore) Get(_ context.Context, tenantID, docType string) (*types.WorkspaceDoc, error) {
	if tenant, ok := m.docs[tenantID]; ok {
		if doc, ok := tenant[docType]; ok {
			return doc, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockWorkspaceStore) Put(_ context.Context, doc *types.WorkspaceDoc) error {
	if m.docs[doc.TenantID] == nil {
		m.docs[doc.TenantID] = make(map[string]*types.WorkspaceDoc)
	}
	m.docs[doc.TenantID][doc.DocType] = doc
	return nil
}

func (m *mockWorkspaceStore) List(_ context.Context, _ string) ([]*types.WorkspaceDoc, error) {
	return nil, nil
}

func (m *mockWorkspaceStore) Delete(_ context.Context, _, _ string) error {
	return nil
}

// --- mock UserWorkspaceStore ---

type mockUserWorkspaceStore struct {
	docs map[string]*types.UserWorkspaceDoc // "tenantID:userID:docType" -> doc
}

func newMockUserWorkspaceStore() *mockUserWorkspaceStore {
	return &mockUserWorkspaceStore{docs: make(map[string]*types.UserWorkspaceDoc)}
}

func (m *mockUserWorkspaceStore) key(tenantID, userID, docType string) string {
	return tenantID + ":" + userID + ":" + docType
}

func (m *mockUserWorkspaceStore) Get(_ context.Context, tenantID, userID, docType string) (*types.UserWorkspaceDoc, error) {
	if doc, ok := m.docs[m.key(tenantID, userID, docType)]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockUserWorkspaceStore) Put(_ context.Context, doc *types.UserWorkspaceDoc) error {
	m.docs[m.key(doc.TenantID, doc.UserID, doc.DocType)] = doc
	return nil
}

func (m *mockUserWorkspaceStore) List(_ context.Context, _, _ string) ([]*types.UserWorkspaceDoc, error) {
	return nil, nil
}

func (m *mockUserWorkspaceStore) Delete(_ context.Context, _, _, _ string) error {
	return nil
}

// --- mock skill stores (for SkillsLoader) ---

type mockSkillRegistryStore struct {
	records map[string]*types.SkillRecord
	byID    map[int64]*types.SkillRecord
}

func newMockSkillRegistryStore() *mockSkillRegistryStore {
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

type mockTenantSkillStore struct {
	installed map[string][]*types.TenantSkill
}

func newMockTenantSkillStore() *mockTenantSkillStore {
	return &mockTenantSkillStore{installed: make(map[string][]*types.TenantSkill)}
}

func (m *mockTenantSkillStore) Install(_ context.Context, ts *types.TenantSkill) error {
	m.installed[ts.TenantID] = append(m.installed[ts.TenantID], ts)
	return nil
}

func (m *mockTenantSkillStore) Uninstall(_ context.Context, _ string, _ int64) error { return nil }

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

func (m *mockObjectStore) Put(_ context.Context, _ string, _ io.Reader) error {
	return nil
}

func (m *mockObjectStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockObjectStore) Delete(_ context.Context, _ string) error { return nil }
func (m *mockObjectStore) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// --- helper ---

func buildTestLoader(t *testing.T, reg *mockSkillRegistryStore) *skills.SkillsLoader {
	t.Helper()
	return skills.NewSkillsLoader(
		reg,
		newMockTenantSkillStore(),
		newMockTenantStore(),
		newMockObjectStore(),
		t.TempDir(),
	)
}

// --- tests ---

func TestInstructionProvider_SystemBase(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()
	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)

	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	ctx := newMockReadonlyContext(map[string]any{
		"tenant_id": "t1",
	})

	result, err := provider(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Should always contain the system base
	if !strings.Contains(result, "ABot Agent") {
		t.Error("missing system base header")
	}
	if !strings.Contains(result, "Runtime") {
		t.Error("missing runtime section")
	}
}

func TestInstructionProvider_TenantPersona(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()

	// Set up tenant workspace docs
	ws.docs["t1"] = map[string]*types.WorkspaceDoc{
		"IDENTITY": {TenantID: "t1", DocType: "IDENTITY", Content: "You are a helpful bot."},
		"RULES":    {TenantID: "t1", DocType: "RULES", Content: "Be concise."},
	}

	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)
	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	ctx := newMockReadonlyContext(map[string]any{"tenant_id": "t1"})
	result, err := provider(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "You are a helpful bot.") {
		t.Error("missing IDENTITY content")
	}
	if !strings.Contains(result, "Be concise.") {
		t.Error("missing RULES content")
	}
}

func TestInstructionProvider_UserContext(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()

	uws.docs["t1:u1:USER"] = &types.UserWorkspaceDoc{
		TenantID: "t1", UserID: "u1", DocType: "USER",
		Content: "Prefers dark mode.",
	}

	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)
	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	ctx := newMockReadonlyContext(map[string]any{
		"tenant_id": "t1",
		"user_id":   "u1",
	})
	result, err := provider(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Prefers dark mode.") {
		t.Error("missing user profile")
	}
}

func TestInstructionProvider_TenantIsolation(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()

	ws.docs["t1"] = map[string]*types.WorkspaceDoc{
		"IDENTITY": {TenantID: "t1", DocType: "IDENTITY", Content: "Tenant 1 identity"},
	}
	ws.docs["t2"] = map[string]*types.WorkspaceDoc{
		"IDENTITY": {TenantID: "t2", DocType: "IDENTITY", Content: "Tenant 2 identity"},
	}

	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)
	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	// Tenant 1 should only see its own identity
	ctx1 := newMockReadonlyContext(map[string]any{"tenant_id": "t1"})
	r1, _ := provider(ctx1)
	if !strings.Contains(r1, "Tenant 1 identity") {
		t.Error("t1 missing its identity")
	}
	if strings.Contains(r1, "Tenant 2 identity") {
		t.Error("t1 should not see t2 identity")
	}

	// Tenant 2 should only see its own identity
	ctx2 := newMockReadonlyContext(map[string]any{"tenant_id": "t2"})
	r2, _ := provider(ctx2)
	if !strings.Contains(r2, "Tenant 2 identity") {
		t.Error("t2 missing its identity")
	}
	if strings.Contains(r2, "Tenant 1 identity") {
		t.Error("t2 should not see t1 identity")
	}
}

func TestInstructionProvider_UserPersonaOverride(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()

	// Tenant-level template
	ws.docs["t1"] = map[string]*types.WorkspaceDoc{
		"IDENTITY": {TenantID: "t1", DocType: "IDENTITY", Content: "Tenant default"},
	}
	// User-level override
	uws.docs["t1:u1:IDENTITY"] = &types.UserWorkspaceDoc{
		TenantID: "t1", UserID: "u1", DocType: "IDENTITY",
		Content: "User custom",
	}

	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)
	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	ctx := newMockReadonlyContext(map[string]any{
		"tenant_id": "t1",
		"user_id":   "u1",
	})
	result, err := provider(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "User custom") {
		t.Error("expected user-level IDENTITY override, got tenant default")
	}
	if strings.Contains(result, "Tenant default") {
		t.Error("tenant-level IDENTITY should be shadowed by user override")
	}
}

func TestInstructionProvider_UserPersonaFallback(t *testing.T) {
	ws := newMockWorkspaceStore()
	uws := newMockUserWorkspaceStore()

	// Tenant-level template only, no user-level SOUL
	ws.docs["t1"] = map[string]*types.WorkspaceDoc{
		"SOUL": {TenantID: "t1", DocType: "SOUL", Content: "Tenant soul"},
	}

	reg := newMockSkillRegistryStore()
	loader := buildTestLoader(t, reg)
	cb := workspace.NewContextBuilder(ws, uws, loader, nil, nil, nil, nil)
	provider := cb.InstructionProvider()

	ctx := newMockReadonlyContext(map[string]any{
		"tenant_id": "t1",
		"user_id":   "u1",
	})
	result, err := provider(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Tenant soul") {
		t.Error("expected fallback to tenant-level SOUL when user has none")
	}
}

func TestTrimLayers_NoTrimNeeded(t *testing.T) {
	layers := []workspace.PromptLayer{
		workspace.NewPromptLayer("short", 0),
		workspace.NewPromptLayer("also short", 1),
	}
	result := workspace.TrimLayers(layers, 10000)
	if len(result) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(result))
	}
}

func TestTrimLayers_DropsLowestPriority(t *testing.T) {
	layers := []workspace.PromptLayer{
		workspace.NewPromptLayer(strings.Repeat("a", 100), 0), // system base — never dropped
		workspace.NewPromptLayer(strings.Repeat("b", 100), 1), // tenant persona
		workspace.NewPromptLayer(strings.Repeat("c", 100), 5), // skills — lowest priority
		workspace.NewPromptLayer(strings.Repeat("d", 100), 3), // user context
	}
	// Total ~400 chars + 3 separators (7 each) = 421. Set limit to 320 to force exactly one drop.
	result := workspace.TrimLayers(layers, 320)

	// Skills (priority 5) should be dropped first.
	for _, l := range result {
		if l.Priority() == 5 {
			t.Error("skills layer (priority 5) should have been dropped")
		}
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 layers after trim, got %d", len(result))
	}
}

func TestTrimLayers_NeverDropsPriorityZero(t *testing.T) {
	layers := []workspace.PromptLayer{
		workspace.NewPromptLayer(strings.Repeat("x", 500), 0), // system base — priority 0
	}
	// Even if the single layer exceeds the limit, priority 0 is never dropped.
	result := workspace.TrimLayers(layers, 100)
	if len(result) != 1 {
		t.Fatalf("priority-0 layer should never be dropped, got %d layers", len(result))
	}
}

func TestTrimLayers_DropsMultiple(t *testing.T) {
	layers := []workspace.PromptLayer{
		workspace.NewPromptLayer(strings.Repeat("a", 100), 0),
		workspace.NewPromptLayer(strings.Repeat("b", 100), 1),
		workspace.NewPromptLayer(strings.Repeat("c", 100), 5),
		workspace.NewPromptLayer(strings.Repeat("d", 100), 4),
		workspace.NewPromptLayer(strings.Repeat("e", 100), 3),
	}
	// Force dropping to just 2 layers.
	result := workspace.TrimLayers(layers, 220)

	// Should keep priority 0 and 1, drop 3, 4, 5.
	if len(result) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(result))
	}
	for _, l := range result {
		if l.Priority() > 1 {
			t.Errorf("layer with priority %d should have been dropped", l.Priority())
		}
	}
}
