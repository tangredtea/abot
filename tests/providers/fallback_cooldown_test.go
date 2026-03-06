package providers_test

import (
	"testing"
	"time"

	"abot/pkg/providers/fallback"
	"abot/pkg/types"
)

func newTestTracker(now time.Time) *fallback.CooldownTracker {
	ct := fallback.NewCooldownTracker()
	ct.SetNowFunc(func() time.Time { return now })
	return ct
}

func TestCalculateCooldown(t *testing.T) {
	tests := []struct {
		count int
		want  time.Duration
	}{
		{0, 0},
		{1, 1 * time.Minute},  // 5^0 = 1
		{2, 5 * time.Minute},  // 5^1 = 5
		{3, 25 * time.Minute}, // 5^2 = 25
		{4, time.Hour},        // 5^3 = 125min, capped to 1h
		{5, time.Hour},        // 5^3 = 125min, capped to 1h (exp capped at 3)
		{100, time.Hour},      // still capped
	}
	for _, tt := range tests {
		got := fallback.CalculateCooldown(tt.count)
		if got != tt.want {
			t.Errorf("CalculateCooldown(%d) = %v, want %v", tt.count, got, tt.want)
		}
	}
}

func TestMarkFailureAndIsAvailable(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ct := newTestTracker(now)

	// Initially available.
	if !ct.IsAvailable("openai") {
		t.Fatal("expected openai to be available initially")
	}

	// Mark failure → enters 1min cooldown.
	ct.MarkFailure("openai", types.FailoverRateLimit)
	if ct.IsAvailable("openai") {
		t.Fatal("expected openai to be in cooldown after failure")
	}

	// Advance past cooldown.
	ct.SetNowFunc(func() time.Time { return now.Add(2 * time.Minute) })
	if !ct.IsAvailable("openai") {
		t.Fatal("expected openai to be available after cooldown expires")
	}
}

func TestMarkSuccessResetsCooldown(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ct := newTestTracker(now)

	ct.MarkFailure("anthropic", types.FailoverTimeout)
	if ct.IsAvailable("anthropic") {
		t.Fatal("expected cooldown after failure")
	}

	ct.MarkSuccess("anthropic")
	if !ct.IsAvailable("anthropic") {
		t.Fatal("expected available after MarkSuccess")
	}
}

func TestCooldownRemaining(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ct := newTestTracker(now)

	// No entry → 0.
	if r := ct.CooldownRemaining("unknown"); r != 0 {
		t.Fatalf("expected 0 remaining, got %v", r)
	}

	ct.MarkFailure("openai", types.FailoverRateLimit)

	// Advance 30s into 1min cooldown.
	ct.SetNowFunc(func() time.Time { return now.Add(30 * time.Second) })
	r := ct.CooldownRemaining("openai")
	if r < 29*time.Second || r > 31*time.Second {
		t.Fatalf("expected ~30s remaining, got %v", r)
	}

	// Advance past cooldown.
	ct.SetNowFunc(func() time.Time { return now.Add(2 * time.Minute) })
	if r := ct.CooldownRemaining("openai"); r != 0 {
		t.Fatalf("expected 0 remaining after expiry, got %v", r)
	}
}

func TestExponentialBackoff(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ct := newTestTracker(now)

	// 1st failure → 1min cooldown.
	ct.MarkFailure("p", types.FailoverRateLimit)
	r1 := ct.CooldownRemaining("p")

	// 2nd failure → 5min cooldown.
	ct.SetNowFunc(func() time.Time { return now.Add(2 * time.Minute) })
	ct.MarkFailure("p", types.FailoverRateLimit)
	r2 := ct.CooldownRemaining("p")

	if r2 <= r1 {
		t.Fatalf("expected increasing cooldown: r1=%v r2=%v", r1, r2)
	}
}
