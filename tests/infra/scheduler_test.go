package infra_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"abot/pkg/bus"
	"abot/pkg/scheduler"
	"abot/pkg/types"
)

// mockSchedulerStore is an in-memory SchedulerStore for testing.
type mockSchedulerStore struct {
	mu   sync.Mutex
	jobs map[string]*types.CronJob
}

func newMockSchedulerStore() *mockSchedulerStore {
	return &mockSchedulerStore{jobs: make(map[string]*types.CronJob)}
}

func (m *mockSchedulerStore) SaveJob(_ context.Context, job *types.CronJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockSchedulerStore) ListJobs(_ context.Context, tenantID string) ([]*types.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*types.CronJob
	for _, j := range m.jobs {
		if tenantID == "" || j.TenantID == tenantID {
			out = append(out, j)
		}
	}
	return out, nil
}

func (m *mockSchedulerStore) DeleteJob(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, jobID)
	return nil
}

func (m *mockSchedulerStore) UpdateJobState(_ context.Context, jobID string, state *types.CronJobState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[jobID]; ok {
		j.State = *state
	}
	return nil
}

// --- CronService lifecycle tests ---

func TestCronService_StartStop(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestCronService_AddAndListJobs(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	ctx := context.Background()
	err := svc.AddJob(ctx, &types.CronJob{
		ID:       "job1",
		TenantID: "t1",
		Name:     "test job",
		Enabled:  true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 9 * * *"},
		Message:  "hello",
		Channel:  "cli",
	})
	if err != nil {
		t.Fatal(err)
	}

	jobs := svc.ListJobs("t1")
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "test job" {
		t.Errorf("name: %q", jobs[0].Name)
	}
}

func TestCronService_RemoveJob(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	ctx := context.Background()
	svc.AddJob(ctx, &types.CronJob{
		ID: "j1", TenantID: "t1", Enabled: true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 9 * * *"},
	})

	if err := svc.RemoveJob(ctx, "j1"); err != nil {
		t.Fatal(err)
	}
	if len(svc.ListJobs("t1")) != 0 {
		t.Error("expected 0 jobs after remove")
	}
}

func TestCronService_EnableDisable(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	ctx := context.Background()
	svc.AddJob(ctx, &types.CronJob{
		ID: "j1", TenantID: "t1", Enabled: true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 9 * * *"},
	})

	if err := svc.EnableJob(ctx, "j1", false); err != nil {
		t.Fatal(err)
	}
	jobs := svc.ListJobs("t1")
	if len(jobs) != 1 || jobs[0].Enabled {
		t.Error("expected job to be disabled")
	}

	if err := svc.EnableJob(ctx, "j1", true); err != nil {
		t.Fatal(err)
	}
	jobs = svc.ListJobs("t1")
	if !jobs[0].Enabled {
		t.Error("expected job to be re-enabled")
	}
}

func TestCronService_EnableJob_NotFound(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	err := svc.EnableJob(context.Background(), "nonexistent", true)
	if err == nil {
		t.Error("expected error for missing job")
	}
}

func TestCronService_Status(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	ctx := context.Background()
	svc.AddJob(ctx, &types.CronJob{
		ID: "j1", TenantID: "t1", Enabled: true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 9 * * *"},
	})
	svc.AddJob(ctx, &types.CronJob{
		ID: "j2", TenantID: "t1", Enabled: false,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 10 * * *"},
	})

	st := svc.Status()
	if st.TotalJobs != 2 {
		t.Errorf("total: %d", st.TotalJobs)
	}
	if st.EnabledJobs != 1 {
		t.Errorf("enabled: %d", st.EnabledJobs)
	}
	if st.NextWakeAt == nil {
		t.Error("expected NextWakeAt to be set")
	}
}

func TestCronService_ListJobs_FilterByTenant(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	ctx := context.Background()
	svc.AddJob(ctx, &types.CronJob{
		ID: "j1", TenantID: "t1", Enabled: true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 9 * * *"},
	})
	svc.AddJob(ctx, &types.CronJob{
		ID: "j2", TenantID: "t2", Enabled: true,
		Schedule: types.CronSchedule{Kind: types.ScheduleCron, Expr: "0 10 * * *"},
	})

	if len(svc.ListJobs("t1")) != 1 {
		t.Error("expected 1 job for t1")
	}
	if len(svc.ListJobs("")) != 2 {
		t.Error("expected 2 jobs for all tenants")
	}
}

// --- Unique tests from pkg/scheduler/cron_test.go ---

func TestCronService_ScheduleAt(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(100)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	job := &types.CronJob{
		ID:       "at-1",
		TenantID: "t1",
		Name:     "once",
		Enabled:  true,
		Schedule: types.CronSchedule{
			Kind:   types.ScheduleAt,
			AtTime: time.Now().Add(50 * time.Millisecond),
		},
		Message: "fire once",
		Channel: "cli",
		ChatID:  "c1",
	}
	if err := svc.AddJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	// Consume the fired message from the real bus.
	got, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "fire once" {
		t.Fatalf("got content %q, want %q", got.Content, "fire once")
	}
	if got.TenantID != "t1" {
		t.Fatalf("got tenant %q, want %q", got.TenantID, "t1")
	}
}

func TestCronService_ScheduleEvery(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(100)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	job := &types.CronJob{
		ID:       "every-1",
		TenantID: "t1",
		Name:     "repeat",
		Enabled:  true,
		Schedule: types.CronSchedule{
			Kind:     types.ScheduleEvery,
			Interval: 80 * time.Millisecond,
		},
		Message: "ping",
		Channel: "cli",
	}
	if err := svc.AddJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	// Wait for at least 2 fires.
	for i := 0; i < 2; i++ {
		got, err := b.ConsumeInbound(ctx)
		if err != nil {
			t.Fatalf("fire %d: consume error: %v", i, err)
		}
		if got.Content != "ping" {
			t.Fatalf("fire %d: got %q, want %q", i, got.Content, "ping")
		}
	}
}

func TestCronService_ScheduleCron(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(10)
	defer b.Close()

	svc := scheduler.New(store, b, nil)

	// Verify ComputeNextRun for cron expressions without starting the loop.
	job := &types.CronJob{
		ID:       "cron-1",
		TenantID: "t1",
		Enabled:  true,
		Schedule: types.CronSchedule{
			Kind: types.ScheduleCron,
			Expr: "0 9 * * *", // daily at 09:00
		},
	}

	ref := time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC)
	next := svc.ComputeNextRun(job, ref)
	want := time.Date(2026, 2, 25, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next=%v, want=%v", next, want)
	}

	// After 09:00, next should be tomorrow.
	ref2 := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	next2 := svc.ComputeNextRun(job, ref2)
	want2 := time.Date(2026, 2, 26, 9, 0, 0, 0, time.UTC)
	if !next2.Equal(want2) {
		t.Fatalf("next2=%v, want=%v", next2, want2)
	}
}

func TestCronService_DeleteAfterRun(t *testing.T) {
	store := newMockSchedulerStore()
	b := bus.New(100)
	defer b.Close()

	svc := scheduler.New(store, b, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()

	job := &types.CronJob{
		ID:             "del-1",
		TenantID:       "t1",
		Enabled:        true,
		DeleteAfterRun: true,
		Schedule: types.CronSchedule{
			Kind:   types.ScheduleAt,
			AtTime: time.Now().Add(30 * time.Millisecond),
		},
		Message: "bye",
		Channel: "cli",
	}
	if err := svc.AddJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	// Wait for fire.
	if _, err := b.ConsumeInbound(ctx); err != nil {
		t.Fatal(err)
	}

	// Job should be removed from memory and store.
	time.Sleep(50 * time.Millisecond)
	if jobs := svc.ListJobs(""); len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

// --- Mock types for heartbeat tests ---

type mockTenantStore struct {
	mu      sync.Mutex
	tenants []*types.Tenant
}

func (s *mockTenantStore) Get(_ context.Context, id string) (*types.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tenants {
		if t.TenantID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (s *mockTenantStore) Put(_ context.Context, t *types.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants = append(s.tenants, t)
	return nil
}

func (s *mockTenantStore) List(_ context.Context, _ string) ([]*types.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]*types.Tenant, len(s.tenants))
	copy(cp, s.tenants)
	return cp, nil
}

func (s *mockTenantStore) Delete(context.Context, string) error { return nil }

type mockWorkspaceStore struct {
	mu   sync.Mutex
	docs map[string]*types.WorkspaceDoc
}

func newMockWorkspaceStore() *mockWorkspaceStore {
	return &mockWorkspaceStore{docs: make(map[string]*types.WorkspaceDoc)}
}

func (s *mockWorkspaceStore) Get(_ context.Context, tenantID, docType string) (*types.WorkspaceDoc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[tenantID+":"+docType], nil
}

func (s *mockWorkspaceStore) Put(_ context.Context, doc *types.WorkspaceDoc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[doc.TenantID+":"+doc.DocType] = doc
	return nil
}

func (s *mockWorkspaceStore) List(context.Context, string) ([]*types.WorkspaceDoc, error) {
	return nil, nil
}

func (s *mockWorkspaceStore) Delete(context.Context, string, string) error { return nil }

// --- Heartbeat tests from pkg/scheduler/heartbeat_test.go ---

func TestHeartbeat_TickPublishesForTenantsWithHeartbeat(t *testing.T) {
	b := bus.New(100)
	defer b.Close()

	tenants := &mockTenantStore{
		tenants: []*types.Tenant{
			{TenantID: "t1", Name: "Forum A"},
			{TenantID: "t2", Name: "Forum B"},
			{TenantID: "t3", Name: "Forum C"},
		},
	}
	ws := newMockWorkspaceStore()
	// Only t1 and t3 have HEARTBEAT docs.
	_ = ws.Put(context.Background(), &types.WorkspaceDoc{
		TenantID: "t1", DocType: "HEARTBEAT",
		Content: "Check new posts",
	})
	_ = ws.Put(context.Background(), &types.WorkspaceDoc{
		TenantID: "t3", DocType: "HEARTBEAT",
		Content: "Summarize activity",
	})

	hb := scheduler.NewHeartbeat(scheduler.HeartbeatConfig{
		Bus:            b,
		WorkspaceStore: ws,
		Tenants:        tenants,
		Interval:       100 * time.Millisecond,
		Channel:        "heartbeat",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := hb.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer hb.Stop()

	// Collect messages from first tick.
	got := make(map[string]string) // tenantID -> content
	deadline := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case <-deadline:
			t.Fatalf("timeout: got %d messages, want 2", len(got))
		default:
			msg, err := b.ConsumeInbound(ctx)
			if err != nil {
				t.Fatal(err)
			}
			got[msg.TenantID] = msg.Content
		}
	}

	if got["t1"] != "Check new posts" {
		t.Errorf("t1: got %q", got["t1"])
	}
	if got["t3"] != "Summarize activity" {
		t.Errorf("t3: got %q", got["t3"])
	}
	if _, ok := got["t2"]; ok {
		t.Error("t2 should not have received a heartbeat")
	}
}
