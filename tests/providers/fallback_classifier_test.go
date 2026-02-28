package providers_test

import (
	"context"
	"errors"
	"testing"

	"abot/pkg/providers/fallback"
	"abot/pkg/types"
)

func TestClassifyError_Nil(t *testing.T) {
	if got := fallback.ClassifyError(nil, "p", "m"); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}

func TestClassifyError_ContextCanceled(t *testing.T) {
	if got := fallback.ClassifyError(context.Canceled, "p", "m"); got != nil {
		t.Fatalf("expected nil for context.Canceled, got %v", got)
	}
}

func TestClassifyError_DeadlineExceeded(t *testing.T) {
	got := fallback.ClassifyError(context.DeadlineExceeded, "p", "m")
	if got == nil {
		t.Fatal("expected FailoverError for DeadlineExceeded")
	}
	if got.Reason != types.FailoverTimeout {
		t.Fatalf("expected FailoverTimeout, got %s", got.Reason)
	}
}

func TestClassifyError_ByMessage(t *testing.T) {
	tests := []struct {
		msg    string
		reason types.FailoverReason
	}{
		{"rate limit exceeded", types.FailoverRateLimit},
		{"too many requests", types.FailoverRateLimit},
		{"429 Too Many Requests", types.FailoverRateLimit},
		{"exceeded your current quota", types.FailoverRateLimit},
		{"server overloaded", types.FailoverRateLimit},
		{"overloaded_error", types.FailoverRateLimit},
		{"request timeout", types.FailoverTimeout},
		{"connection timed out", types.FailoverTimeout},
		{"deadline exceeded", types.FailoverTimeout},
		{"invalid api key", types.FailoverAuth},
		{"unauthorized", types.FailoverAuth},
		{"forbidden", types.FailoverAuth},
		{"payment required", types.FailoverBilling},
		{"insufficient credits", types.FailoverBilling},
		{"insufficient balance", types.FailoverBilling},
		{"invalid request format", types.FailoverFormat},
		{"tool_use.id is required", types.FailoverFormat},
		{"string should match pattern", types.FailoverFormat},
	}
	for _, tt := range tests {
		got := fallback.ClassifyError(errors.New(tt.msg), "p", "m")
		if got == nil {
			t.Errorf("ClassifyError(%q) = nil, want reason=%s", tt.msg, tt.reason)
			continue
		}
		if got.Reason != tt.reason {
			t.Errorf("ClassifyError(%q).Reason = %s, want %s", tt.msg, got.Reason, tt.reason)
		}
	}
}
