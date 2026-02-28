package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	adksession "google.golang.org/adk/session"

	abotsession "abot/pkg/session"
	vectordb "abot/pkg/storage/vectordb"
	"abot/pkg/storage/vectordb/qdrant"
	"abot/pkg/types"
)

// ---------- 1. Session JSONL persistence ----------

func TestSessionJSONL_MultiTurn(t *testing.T) {
	dir := t.TempDir()

	svc, err := abotsession.NewJSONLService(dir)
	if err != nil {
		t.Fatalf("NewJSONLService: %v", err)
	}

	ctx := context.Background()
	createResp, err := svc.Create(ctx, &adksession.CreateRequest{
		AppName: "test-app",
		UserID:  "user-1",
		State:   map[string]any{"tenant_id": "default"},
	})
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	sess := createResp.Session
	sessionID := sess.ID()
	t.Logf("Session created: id=%s", sessionID)

	// Simulate 5 turns of conversation.
	for i := 1; i <= 5; i++ {
		event := &adksession.Event{
			ID:        fmt.Sprintf("evt-%d", i),
			Author:    "user",
			Timestamp: time.Now(),
		}
		if err := svc.AppendEvent(ctx, sess, event); err != nil {
			t.Fatalf("AppendEvent turn %d: %v", i, err)
		}
	}

	// Verify events in memory.
	getResp, err := svc.Get(ctx, &adksession.GetRequest{
		AppName:   "test-app",
		UserID:    "user-1",
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("Get session: %v", err)
	}
	events := getResp.Session.Events()
	t.Logf("Events in memory: %d", events.Len())
	if events.Len() != 5 {
		t.Errorf("expected 5 events, got %d", events.Len())
	}

	// Verify JSONL file exists on disk.
	jsonlPath := filepath.Join(dir, "test-app", "user-1_"+sessionID+".jsonl")
	info, err := os.Stat(jsonlPath)
	if err != nil {
		t.Fatalf("JSONL file not found: %v", err)
	}
	t.Logf("JSONL file: %s (%d bytes)", jsonlPath, info.Size())

	// Simulate restart: create new service from same dir.
	svc2, err := abotsession.NewJSONLService(dir)
	if err != nil {
		t.Fatalf("NewJSONLService (reload): %v", err)
	}

	getResp2, err := svc2.Get(ctx, &adksession.GetRequest{
		AppName:   "test-app",
		UserID:    "user-1",
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("Get session after reload: %v", err)
	}
	reloaded := getResp2.Session.Events()
	t.Logf("Events after reload: %d", reloaded.Len())
	if reloaded.Len() != 5 {
		t.Errorf("expected 5 events after reload, got %d", reloaded.Len())
	}
}

// ---------- 2. Vector embed + Qdrant search ----------

func TestVector_EmbedAndSearch(t *testing.T) {
	qdrantAddr := os.Getenv("QDRANT_ADDR")
	if qdrantAddr == "" {
		qdrantAddr = "localhost:6334"
	}
	ollamaBase := os.Getenv("OLLAMA_BASE")
	if ollamaBase == "" {
		ollamaBase = "http://localhost:11434/v1"
	}

	ctx := context.Background()

	store, err := qdrant.New(qdrant.Config{
		Addr:      qdrantAddr,
		Dimension: 768,
	})
	if err != nil {
		t.Skipf("Qdrant not available at %s: %v", qdrantAddr, err)
	}
	defer store.Close()

	collection := fmt.Sprintf("test_abot_%d", time.Now().UnixNano())
	if err := store.EnsureCollection(ctx, collection); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	embedder := vectordb.NewOpenAIEmbedder(vectordb.OpenAIEmbedderConfig{
		BaseURL:   ollamaBase,
		APIKey:    "ollama",
		Model:     "nomic-embed-text",
		Dimension: 768,
	})

	docs := []string{
		"Go is a statically typed programming language designed at Google.",
		"Python is a high-level interpreted programming language.",
		"Qdrant is a vector similarity search engine written in Rust.",
		"ABot is an AI agent framework built with ADK-Go.",
		"The weather in Beijing today is sunny and warm.",
	}

	vectors, err := embedder.Embed(ctx, docs)
	if err != nil {
		t.Skipf("Ollama embedding failed (service down?): %v", err)
	}
	if len(vectors) != len(docs) {
		t.Fatalf("expected %d vectors, got %d", len(docs), len(vectors))
	}

	entries := make([]types.VectorEntry, len(docs))
	for i, doc := range docs {
		entries[i] = types.VectorEntry{
			ID:      uuid.NewString(),
			Vector:  vectors[i],
			Payload: map[string]any{"text": doc, "index": i},
		}
	}
	if err := store.Upsert(ctx, collection, entries); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	queryVecs, err := embedder.Embed(ctx, []string{"AI agent framework in Go"})
	if err != nil {
		t.Fatalf("Embed query: %v", err)
	}

	results, err := store.Search(ctx, collection, &types.VectorSearchRequest{
		Vector: queryVecs[0],
		TopK:   3,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
}
