package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"

	"abot/pkg/storage/vectordb"
	"abot/pkg/storage/vectordb/qdrant"
	"abot/pkg/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ALL CHECKS PASSED")
}

func run() error {
	ctx := context.Background()

	// 1. Connect to Qdrant.
	fmt.Print("[1] Connecting to Qdrant (localhost:6334)... ")
	vs, err := qdrant.New(qdrant.Config{Addr: "localhost:6334", Dimension: 768})
	if err != nil {
		return fmt.Errorf("qdrant connect: %w", err)
	}
	defer vs.Close()
	fmt.Println("OK")

	// 2. Ensure test collection.
	fmt.Print("[2] Ensuring collection 'vectest'... ")
	if err := vs.EnsureCollection(ctx, "vectest"); err != nil {
		return fmt.Errorf("ensure collection: %w", err)
	}
	fmt.Println("OK")

	// 3. Test embedding.
	fmt.Print("[3] Calling Ollama embedding (nomic-embed-text)... ")
	emb := vectordb.NewOpenAIEmbedder(vectordb.OpenAIEmbedderConfig{
		BaseURL:   "http://localhost:11434/v1",
		APIKey:    "ollama",
		Model:     "nomic-embed-text",
		Dimension: 768,
	})
	vecs, err := emb.Embed(ctx, []string{"hello world test"})
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) != 768 {
		return fmt.Errorf("unexpected embedding shape: %d vectors, dim=%d", len(vecs), len(vecs[0]))
	}
	fmt.Printf("OK (dim=%d)\n", len(vecs[0]))

	// 4. Upsert a test point.
	fmt.Print("[4] Upserting test vector... ")
	err = vs.Upsert(ctx, "vectest", []types.VectorEntry{{
		ID:      uuid.NewString(),
		Vector:  vecs[0],
		Payload: map[string]any{"text": "hello world test", "scope": "test"},
	}})
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	fmt.Println("OK")

	// 5. Search.
	fmt.Print("[5] Searching for 'hello world'... ")
	qvecs, err := emb.Embed(ctx, []string{"hello world"})
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}
	results, err := vs.Search(ctx, "vectest", &types.VectorSearchRequest{
		Vector: qvecs[0],
		TopK:   3,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results returned")
	}
	fmt.Printf("OK (got %d results, top score=%.4f, text=%q)\n",
		len(results), results[0].Score, results[0].Payload["text"])

	// 6. Cleanup.
	fmt.Print("[6] Cleaning up test vector... ")
	if err := vs.Delete(ctx, "vectest", map[string]any{"scope": "test"}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Println("OK")

	return nil
}
