package infra_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"abot/pkg/session"

	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"
)

// --- JSONLService tests ---

func TestJSONLService_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := svc.Create(ctx, &adksession.CreateRequest{
		AppName: "testapp",
		UserID:  "user1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Session == nil {
		t.Fatal("expected session")
	}

	sid := resp.Session.ID()
	getResp, err := svc.Get(ctx, &adksession.GetRequest{
		AppName:   "testapp",
		UserID:    "user1",
		SessionID: sid,
	})
	if err != nil {
		t.Fatal(err)
	}
	if getResp.Session.ID() != sid {
		t.Errorf("session ID mismatch: %q vs %q", getResp.Session.ID(), sid)
	}
}

func TestJSONLService_List(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "u1"})
	svc.Create(ctx, &adksession.CreateRequest{AppName: "app", UserID: "u1"})

	listResp, err := svc.List(ctx, &adksession.ListRequest{
		AppName: "app",
		UserID:  "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listResp.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(listResp.Sessions))
	}
}

func TestJSONLService_Delete(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := svc.Create(ctx, &adksession.CreateRequest{
		AppName: "app", UserID: "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	sid := resp.Session.ID()

	err = svc.Delete(ctx, &adksession.DeleteRequest{
		AppName: "app", UserID: "u1", SessionID: sid,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Get(ctx, &adksession.GetRequest{
		AppName: "app", UserID: "u1", SessionID: sid,
	})
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestJSONLService_PersistenceAcrossReload(t *testing.T) {
	dir := t.TempDir()

	// Phase 1: create a session
	svc1, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resp, err := svc1.Create(ctx, &adksession.CreateRequest{
		AppName:   "app",
		UserID:    "u1",
		SessionID: "fixed-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	sid := resp.Session.ID()

	// Phase 2: create a new service instance from the same directory
	svc2, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	getResp, err := svc2.Get(ctx, &adksession.GetRequest{
		AppName: "app", UserID: "u1", SessionID: sid,
	})
	if err != nil {
		t.Fatalf("session not found after reload: %v", err)
	}
	if getResp.Session.ID() != sid {
		t.Errorf("session ID mismatch after reload: %q", getResp.Session.ID())
	}
}

func TestJSONLService_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	listResp, err := svc.List(ctx, &adksession.ListRequest{
		AppName: "app", UserID: "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listResp.Sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(listResp.Sessions))
	}
}

// --- Unique tests from pkg/session/jsonl_store_test.go ---

func TestJSONLService_CreateAndGetWithState(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := svc.Create(ctx, &adksession.CreateRequest{
		AppName:   "myapp",
		UserID:    "user1",
		SessionID: "sess1",
		State:     map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.Session.ID() != "sess1" {
		t.Errorf("session ID = %q, want %q", resp.Session.ID(), "sess1")
	}

	got, err := svc.Get(ctx, &adksession.GetRequest{
		AppName:   "myapp",
		UserID:    "user1",
		SessionID: "sess1",
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Session.AppName() != "myapp" {
		t.Errorf("AppName = %q, want %q", got.Session.AppName(), "myapp")
	}
	if got.Session.UserID() != "user1" {
		t.Errorf("UserID = %q, want %q", got.Session.UserID(), "user1")
	}

	val, err := got.Session.State().Get("key")
	if err != nil {
		t.Fatalf("State.Get: %v", err)
	}
	if val != "value" {
		t.Errorf("state[key] = %v, want %q", val, "value")
	}
}

func TestJSONLService_AppendEvent(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	resp, err := svc.Create(ctx, &adksession.CreateRequest{
		AppName:   "myapp",
		UserID:    "user1",
		SessionID: "sess1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	event := &adksession.Event{
		ID:        "evt1",
		Author:    "user",
		Timestamp: time.Now(),
		LLMResponse: model.LLMResponse{
			Partial: false,
		},
	}

	if err := svc.AppendEvent(ctx, resp.Session, event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	got, err := svc.Get(ctx, &adksession.GetRequest{
		AppName:   "myapp",
		UserID:    "user1",
		SessionID: "sess1",
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Session.Events().Len() != 1 {
		t.Fatalf("events len = %d, want 1", got.Session.Events().Len())
	}
	if got.Session.Events().At(0).ID != "evt1" {
		t.Errorf("event ID = %q, want %q", got.Session.Events().At(0).ID, "evt1")
	}

	// Verify JSONL file exists on disk.
	p := filepath.Join(dir, "myapp", "user1_sess1.jsonl")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Errorf("JSONL file not found at %s", p)
	}
}

func TestJSONLService_ListMultiUser(t *testing.T) {
	dir := t.TempDir()
	svc, err := session.NewJSONLService(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, sid := range []string{"sess1", "sess2", "sess3"} {
		_, err := svc.Create(ctx, &adksession.CreateRequest{
			AppName:   "myapp",
			UserID:    "user1",
			SessionID: sid,
		})
		if err != nil {
			t.Fatalf("Create %s: %v", sid, err)
		}
	}

	// Also create one for a different user.
	_, err = svc.Create(ctx, &adksession.CreateRequest{
		AppName:   "myapp",
		UserID:    "user2",
		SessionID: "sess1",
	})
	if err != nil {
		t.Fatalf("Create user2: %v", err)
	}

	// List for user1 should return 3.
	listResp, err := svc.List(ctx, &adksession.ListRequest{
		AppName: "myapp",
		UserID:  "user1",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listResp.Sessions) != 3 {
		t.Errorf("List len = %d, want 3", len(listResp.Sessions))
	}

	// List for all users should return 4.
	allResp, err := svc.List(ctx, &adksession.ListRequest{
		AppName: "myapp",
	})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(allResp.Sessions) != 4 {
		t.Errorf("List all len = %d, want 4", len(allResp.Sessions))
	}
}
