package skills_test

import (
	"testing"
	"time"

	"abot/pkg/skills"
)

func TestSearchCache_ExactHit(t *testing.T) {
	sc := skills.NewSearchCache(10, 5*time.Minute)
	results := []skills.SearchResult{
		{Slug: "s1", Score: 0.9},
		{Slug: "s2", Score: 0.5},
	}
	sc.Put("hello world", results)

	got, ok := sc.Get("hello world")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 2 || got[0].Slug != "s1" {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestSearchCache_SimilarHit(t *testing.T) {
	sc := skills.NewSearchCache(10, 5*time.Minute)
	results := []skills.SearchResult{{Slug: "match", Score: 1.0}}
	sc.Put("kubernetes deployment", results)

	got, ok := sc.Get("kubernetes deployments")
	if !ok {
		t.Fatal("expected similar cache hit")
	}
	if len(got) != 1 || got[0].Slug != "match" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestSearchCache_Miss(t *testing.T) {
	sc := skills.NewSearchCache(10, 5*time.Minute)
	sc.Put("alpha beta gamma", []skills.SearchResult{{Slug: "a"}})

	_, ok := sc.Get("xyz totally different")
	if ok {
		t.Fatal("expected cache miss for unrelated query")
	}
}

func TestSearchCache_TTLExpiry(t *testing.T) {
	sc := skills.NewSearchCache(10, 50*time.Millisecond)
	sc.Put("expiring", []skills.SearchResult{{Slug: "e"}})

	time.Sleep(80 * time.Millisecond)

	_, ok := sc.Get("expiring")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestSearchCache_LRUEviction(t *testing.T) {
	sc := skills.NewSearchCache(2, 5*time.Minute)
	sc.Put("first", []skills.SearchResult{{Slug: "1"}})
	sc.Put("second", []skills.SearchResult{{Slug: "2"}})
	sc.Put("third", []skills.SearchResult{{Slug: "3"}})

	// "first" should be evicted
	_, ok := sc.Get("first")
	if ok {
		t.Fatal("expected 'first' to be evicted")
	}

	if _, ok := sc.Get("second"); !ok {
		t.Fatal("expected 'second' to exist")
	}
	if _, ok := sc.Get("third"); !ok {
		t.Fatal("expected 'third' to exist")
	}
}

func TestSearchCache_EmptyQuery(t *testing.T) {
	sc := skills.NewSearchCache(10, 5*time.Minute)
	sc.Put("", []skills.SearchResult{{Slug: "x"}})

	if sc.Len() != 0 {
		t.Fatal("empty query should not be cached")
	}

	_, ok := sc.Get("")
	if ok {
		t.Fatal("empty query should miss")
	}
}

func TestSearchCache_CopyIsolation(t *testing.T) {
	sc := skills.NewSearchCache(10, 5*time.Minute)
	original := []skills.SearchResult{{Slug: "orig", Score: 1.0}}
	sc.Put("test", original)

	// Mutating original slice should not affect cache
	original[0].Slug = "mutated"

	got, ok := sc.Get("test")
	if !ok {
		t.Fatal("expected hit")
	}
	if got[0].Slug != "orig" {
		t.Fatal("cache should be isolated from caller mutations")
	}
}

func TestBuildTrigrams(t *testing.T) {
	tri := skills.BuildTrigrams("hello")
	if len(tri) == 0 {
		t.Fatal("expected trigrams for 'hello'")
	}

	// Short string should not produce trigrams
	if tri := skills.BuildTrigrams("ab"); tri != nil {
		t.Fatal("expected nil for short string")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	a := skills.BuildTrigrams("hello world")
	b := skills.BuildTrigrams("hello world")
	if sim := skills.JaccardSimilarity(a, b); sim != 1.0 {
		t.Fatalf("identical strings: got %f, want 1.0", sim)
	}

	// Empty sets
	if sim := skills.JaccardSimilarity(nil, nil); sim != 1.0 {
		t.Fatalf("both empty: got %f, want 1.0", sim)
	}
}
