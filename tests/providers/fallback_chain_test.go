package providers_test

import (
	"context"
	"errors"
	"testing"

	"abot/pkg/providers/fallback"
	"abot/pkg/types"
)

func TestChain_NoCandidates(t *testing.T) {
	c := fallback.NewChain(fallback.NewCooldownTracker())
	_, err := c.Execute(context.Background(), nil, func(ctx context.Context, p, m string) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestChain_FirstSucceeds(t *testing.T) {
	c := fallback.NewChain(fallback.NewCooldownTracker())
	candidates := []fallback.Candidate{
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude-3"},
	}

	called := 0
	result, err := c.Execute(context.Background(), candidates, func(ctx context.Context, p, m string) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
	if result.Provider != "openai" {
		t.Fatalf("expected provider=openai, got %s", result.Provider)
	}
}

func TestChain_FallbackOnRetriableError(t *testing.T) {
	c := fallback.NewChain(fallback.NewCooldownTracker())
	candidates := []fallback.Candidate{
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude-3"},
	}

	call := 0
	result, err := c.Execute(context.Background(), candidates, func(ctx context.Context, p, m string) error {
		call++
		if call == 1 {
			return errors.New("rate limit exceeded")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call != 2 {
		t.Fatalf("expected 2 calls, got %d", call)
	}
	if result.Provider != "anthropic" {
		t.Fatalf("expected fallback to anthropic, got %s", result.Provider)
	}
}

func TestChain_AbortOnNonRetriable(t *testing.T) {
	c := fallback.NewChain(fallback.NewCooldownTracker())
	candidates := []fallback.Candidate{
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude-3"},
	}

	call := 0
	_, err := c.Execute(context.Background(), candidates, func(ctx context.Context, p, m string) error {
		call++
		return errors.New("invalid request format")
	})
	if err == nil {
		t.Fatal("expected error for non-retriable")
	}
	if call != 1 {
		t.Fatalf("expected 1 call (no fallback for format error), got %d", call)
	}
	var fe *fallback.FailoverError
	if !errors.As(err, &fe) {
		t.Fatalf("expected FailoverError, got %T", err)
	}
	if fe.Reason != types.FailoverFormat {
		t.Fatalf("expected FailoverFormat, got %s", fe.Reason)
	}
}
