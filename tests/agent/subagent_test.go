package agent_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/adk/session"

	"abot/pkg/agent"
	"abot/pkg/types"
)

func TestSubagentManager_SpawnAndGetTask(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	r, a := newTestRunner(t, ss, "test-app", "worker", "task done")
	reg.Register(&agent.AgentEntry{
		ID:     "worker",
		Agent:  a,
		Runner: r,
		Config: types.AgentDefinition{ID: "worker", Name: "worker"},
	})

	sm := agent.NewSubagentManager(reg, mb, ss, "test-app")

	taskID, err := sm.Spawn(context.Background(), "do something", "worker", "cli", "c1", "default", "user")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if taskID == "" {
		t.Fatal("expected non-empty task ID")
	}

	// Wait for completion.
	deadline := time.After(5 * time.Second)
	for {
		st, ok := sm.GetTask(taskID)
		if !ok {
			t.Fatal("task not found")
		}
		if st.Status == "completed" || st.Status == "failed" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for subtask")
		case <-time.After(50 * time.Millisecond):
		}
	}

	st, _ := sm.GetTask(taskID)
	if st.Status != "completed" {
		t.Fatalf("expected completed, got %s: %s", st.Status, st.Result)
	}
}

func TestSubagentManager_SpawnNotFound(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()
	sm := agent.NewSubagentManager(reg, mb, ss, "test-app")

	_, err := sm.Spawn(context.Background(), "task", "nonexistent", "cli", "c1", "default", "user")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestSubagentManager_NotifiesOriginChannel(t *testing.T) {
	ss := session.InMemoryService()
	mb := newMockBus()
	reg := agent.NewAgentRegistry()

	r, a := newTestRunner(t, ss, "test-app", "notifier", "result text")
	reg.Register(&agent.AgentEntry{
		ID:     "notifier",
		Agent:  a,
		Runner: r,
		Config: types.AgentDefinition{ID: "notifier", Name: "notifier"},
	})

	sm := agent.NewSubagentManager(reg, mb, ss, "test-app")
	taskID, _ := sm.Spawn(context.Background(), "notify me", "notifier", "discord", "chat-99", "default", "user")

	// Wait for completion.
	deadline := time.After(5 * time.Second)
	for {
		st, _ := sm.GetTask(taskID)
		if st.Status != "running" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Check outbound notification was published.
	out := mb.getOutbound()
	if len(out) == 0 {
		t.Fatal("expected outbound notification")
	}
	if out[0].Channel != "discord" || out[0].ChatID != "chat-99" {
		t.Fatalf("unexpected outbound: %+v", out[0])
	}
}
