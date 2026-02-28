package agent_test

import (
	"context"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"

	"abot/pkg/agent"
)

func createSessionWithEvents(t *testing.T, ss session.Service, appName, userID, sessionID string, n int) session.Session {
	t.Helper()
	resp, err := ss.Create(context.Background(), &session.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess := resp.Session

	for i := range n {
		ev := session.NewEvent("test")
		ev.Author = "user"
		ev.LLMResponse = model.LLMResponse{
			Content: &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "message " + string(rune('A'+i%26))}},
			},
		}
		if err := ss.AppendEvent(context.Background(), sess, ev); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}
	return sess
}

func TestCompressor_ShouldCompress_EventThreshold(t *testing.T) {
	ss := session.InMemoryService()
	comp := agent.NewCompressor(&mockLLM{response: "summary"}, ss, "test-app")

	sess := createSessionWithEvents(t, ss, "test-app", "u1", "s1", 51)
	if !comp.ShouldCompress(sess, 128000) {
		t.Fatal("expected ShouldCompress=true for 51 events")
	}
}

func TestCompressor_ShouldCompress_BelowThreshold(t *testing.T) {
	ss := session.InMemoryService()
	comp := agent.NewCompressor(&mockLLM{response: "summary"}, ss, "test-app")

	sess := createSessionWithEvents(t, ss, "test-app", "u1", "s2", 10)
	if comp.ShouldCompress(sess, 128000) {
		t.Fatal("expected ShouldCompress=false for 10 events with large window")
	}
}

func TestCompressor_Compress(t *testing.T) {
	ss := session.InMemoryService()
	comp := agent.NewCompressor(&mockLLM{response: "compressed summary"}, ss, "test-app")

	sess := createSessionWithEvents(t, ss, "test-app", "u1", "s3", 20)
	if err := comp.Compress(context.Background(), sess); err != nil {
		t.Fatalf("compress: %v", err)
	}

	got, err := ss.Get(context.Background(), &session.GetRequest{
		AppName:   "test-app",
		UserID:    "u1",
		SessionID: "s3",
	})
	if err != nil {
		t.Fatalf("get after compress: %v", err)
	}

	newLen := got.Session.Events().Len()
	if newLen != 6 {
		t.Fatalf("expected 6 events after compress, got %d", newLen)
	}
}

func TestCompressor_ForceCompress(t *testing.T) {
	ss := session.InMemoryService()
	comp := agent.NewCompressor(&mockLLM{response: "unused"}, ss, "test-app")

	sess := createSessionWithEvents(t, ss, "test-app", "u1", "s4", 20)
	if err := comp.ForceCompress(context.Background(), sess); err != nil {
		t.Fatalf("force compress: %v", err)
	}

	got, err := ss.Get(context.Background(), &session.GetRequest{
		AppName:   "test-app",
		UserID:    "u1",
		SessionID: "s4",
	})
	if err != nil {
		t.Fatalf("get after force compress: %v", err)
	}

	newLen := got.Session.Events().Len()
	if newLen != 11 {
		t.Fatalf("expected 11 events after force compress, got %d", newLen)
	}
}

func TestCompressor_Compress_TooFewEvents(t *testing.T) {
	ss := session.InMemoryService()
	comp := agent.NewCompressor(&mockLLM{response: "summary"}, ss, "test-app")

	sess := createSessionWithEvents(t, ss, "test-app", "u1", "s5", 3)
	if err := comp.Compress(context.Background(), sess); err != nil {
		t.Fatalf("compress: %v", err)
	}

	// With <= 4 events, compress should be a no-op.
	got, err := ss.Get(context.Background(), &session.GetRequest{
		AppName: "test-app", UserID: "u1", SessionID: "s5",
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Session.Events().Len() != 3 {
		t.Fatalf("expected 3 events (no-op), got %d", got.Session.Events().Len())
	}
}
