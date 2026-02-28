package fallback

import (
	"context"
	"fmt"
	"strings"
	"time"

	"abot/pkg/types"
)

// Candidate represents one provider/model to try in the fallback chain.
type Candidate struct {
	Provider string
	Model    string
}

// Attempt records one try in the fallback chain.
type Attempt struct {
	Provider string
	Model    string
	Error    error
	Reason   types.FailoverReason
	Duration time.Duration
	Skipped  bool
}

// Result contains the successful response and metadata about all attempts.
type Result struct {
	Provider string
	Model    string
	Attempts []Attempt
}

// Chain orchestrates model fallback across multiple candidates.
type Chain struct {
	cooldown *CooldownTracker
}

// NewChain creates a fallback chain with the given cooldown tracker.
func NewChain(cooldown *CooldownTracker) *Chain {
	return &Chain{cooldown: cooldown}
}

// Execute tries each candidate in order, respecting cooldowns and error classification.
//
// Behavior:
//   - Candidates in cooldown are skipped.
//   - context.Canceled aborts immediately (no fallback).
//   - Non-retriable errors (format) abort immediately.
//   - Retriable errors trigger fallback to next candidate.
//   - Success resets cooldown for that provider.
//   - If all fail, returns ExhaustedError with all attempts.
func (c *Chain) Execute(
	ctx context.Context,
	candidates []Candidate,
	run func(ctx context.Context, provider, model string) error,
) (*Result, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("fallback: no candidates configured")
	}

	result := &Result{
		Attempts: make([]Attempt, 0, len(candidates)),
	}

	for i, cand := range candidates {
		if ctx.Err() == context.Canceled {
			return nil, context.Canceled
		}

		// Skip providers in cooldown.
		if !c.cooldown.IsAvailable(cand.Provider) {
			remaining := c.cooldown.CooldownRemaining(cand.Provider)
			result.Attempts = append(result.Attempts, Attempt{
				Provider: cand.Provider,
				Model:    cand.Model,
				Skipped:  true,
				Reason:   types.FailoverRateLimit,
				Error: fmt.Errorf("provider %s in cooldown (%s remaining)",
					cand.Provider, remaining.Round(time.Second)),
			})
			continue
		}

		start := time.Now()
		err := run(ctx, cand.Provider, cand.Model)
		elapsed := time.Since(start)

		if err == nil {
			c.cooldown.MarkSuccess(cand.Provider)
			result.Provider = cand.Provider
			result.Model = cand.Model
			return result, nil
		}

		// Context cancellation: abort, no fallback.
		if ctx.Err() == context.Canceled {
			result.Attempts = append(result.Attempts, Attempt{
				Provider: cand.Provider, Model: cand.Model,
				Error: err, Duration: elapsed,
			})
			return nil, context.Canceled
		}

		// Classify the error.
		failErr := ClassifyError(err, cand.Provider, cand.Model)

		if failErr == nil {
			// Unclassifiable: treat as transient, fall through to next candidate.
			result.Attempts = append(result.Attempts, Attempt{
				Provider: cand.Provider, Model: cand.Model,
				Error: err, Reason: types.FailoverTimeout, Duration: elapsed,
			})
			if i == len(candidates)-1 {
				return nil, &ExhaustedError{Attempts: result.Attempts}
			}
			continue
		}

		// Non-retriable: abort.
		if !failErr.IsRetriable() {
			result.Attempts = append(result.Attempts, Attempt{
				Provider: cand.Provider, Model: cand.Model,
				Error: failErr, Reason: failErr.Reason, Duration: elapsed,
			})
			return nil, failErr
		}

		// Retriable: mark failure, try next.
		c.cooldown.MarkFailure(cand.Provider, failErr.Reason)
		result.Attempts = append(result.Attempts, Attempt{
			Provider: cand.Provider, Model: cand.Model,
			Error: failErr, Reason: failErr.Reason, Duration: elapsed,
		})

		if i == len(candidates)-1 {
			return nil, &ExhaustedError{Attempts: result.Attempts}
		}
	}

	// All candidates skipped (all in cooldown).
	return nil, &ExhaustedError{Attempts: result.Attempts}
}

// ExhaustedError indicates all fallback candidates were tried and failed.
type ExhaustedError struct {
	Attempts []Attempt
}

func (e *ExhaustedError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("fallback: all %d candidates failed:", len(e.Attempts)))
	for i, a := range e.Attempts {
		if a.Skipped {
			sb.WriteString(fmt.Sprintf("\n  [%d] %s/%s: skipped (cooldown)",
				i+1, a.Provider, a.Model))
		} else {
			sb.WriteString(fmt.Sprintf("\n  [%d] %s/%s: %v (reason=%s, %s)",
				i+1, a.Provider, a.Model, a.Error, a.Reason,
				a.Duration.Round(time.Millisecond)))
		}
	}
	return sb.String()
}
