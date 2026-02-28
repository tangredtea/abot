package plugins_test

import (
	"sync"
	"testing"

	"abot/pkg/plugins/tokentracker"
)

// --- Token Tracker tests (from tests/plugins) ---

func TestTokenTracker_New(t *testing.T) {
	p, tracker, err := tokentracker.New(tokentracker.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected non-nil plugin")
	}
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
}

func TestTokenTracker_GlobalInitialZero(t *testing.T) {
	_, tracker, _ := tokentracker.New(tokentracker.Config{})
	g := tracker.Global()
	if g.InputTokens.Load() != 0 || g.OutputTokens.Load() != 0 || g.CallCount.Load() != 0 {
		t.Errorf("expected zero counters, got input=%d output=%d calls=%d",
			g.InputTokens.Load(), g.OutputTokens.Load(), g.CallCount.Load())
	}
}

func TestTokenTracker_ByAgent_Unseen(t *testing.T) {
	_, tracker, _ := tokentracker.New(tokentracker.Config{})
	if tracker.ByAgent("nonexistent") != nil {
		t.Error("expected nil for unseen agent")
	}
}

func TestTokenTracker_AllAgents_Empty(t *testing.T) {
	_, tracker, _ := tokentracker.New(tokentracker.Config{})
	all := tracker.AllAgents()
	if len(all) != 0 {
		t.Errorf("expected empty map, got %d entries", len(all))
	}
}

func TestUsageSnapshot(t *testing.T) {
	_, tracker, _ := tokentracker.New(tokentracker.Config{})
	g := tracker.Global()
	g.InputTokens.Add(100)
	g.OutputTokens.Add(50)
	g.CallCount.Add(3)

	snap := g.Snapshot()
	if snap.InputTokens != 100 {
		t.Errorf("input: %d", snap.InputTokens)
	}
	if snap.OutputTokens != 50 {
		t.Errorf("output: %d", snap.OutputTokens)
	}
	if snap.CallCount != 3 {
		t.Errorf("calls: %d", snap.CallCount)
	}
}

// --- Migrated from pkg/plugins/tokentracker (external-package safe) ---

func TestUsage_AtomicConcurrency(t *testing.T) {
	_, tracker, _ := tokentracker.New(tokentracker.Config{})
	g := tracker.Global()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.InputTokens.Add(1)
			g.OutputTokens.Add(2)
			g.CallCount.Add(1)
		}()
	}
	wg.Wait()
	if g.InputTokens.Load() != 100 {
		t.Errorf("expected 100 input tokens, got %d", g.InputTokens.Load())
	}
	if g.OutputTokens.Load() != 200 {
		t.Errorf("expected 200 output tokens, got %d", g.OutputTokens.Load())
	}
}
