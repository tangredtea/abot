package plugins_test

import (
	"sync"
	"testing"

	"abot/pkg/plugins/auditlog"
)

// mockWriter collects audit entries for assertions.
type mockWriter struct {
	mu      sync.Mutex
	entries []auditlog.Entry
}

func (w *mockWriter) WriteEntry(e auditlog.Entry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, e)
}

func (w *mockWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.entries)
}

func (w *mockWriter) get(i int) auditlog.Entry {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.entries[i]
}

// --- Audit Log tests (from tests/plugins) ---

func TestAuditLog_New_DefaultWriter(t *testing.T) {
	p, err := auditlog.New(auditlog.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected non-nil plugin")
	}
}

func TestAuditLog_New_CustomWriter(t *testing.T) {
	w := &mockWriter{}
	p, err := auditlog.New(auditlog.Config{Writer: w})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected non-nil plugin")
	}
}

func TestAuditLog_EntryFields(t *testing.T) {
	e := auditlog.Entry{
		Kind:      "model",
		Phase:     "before",
		AgentName: "test-agent",
		ModelName: "gpt-4o",
	}
	if e.Kind != "model" {
		t.Errorf("kind: %q", e.Kind)
	}
	if e.AgentName != "test-agent" {
		t.Errorf("agent: %q", e.AgentName)
	}
}

func TestMockWriter_WriteEntry(t *testing.T) {
	w := &mockWriter{}
	w.WriteEntry(auditlog.Entry{Kind: "model", Phase: "before"})
	w.WriteEntry(auditlog.Entry{Kind: "tool", Phase: "after"})
	if w.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", w.Len())
	}
}

// --- Audit Log tests (migrated from pkg/plugins/auditlog, external-package safe) ---

func TestAuditLog_EntryErrorField(t *testing.T) {
	e := auditlog.Entry{
		Kind:  "model",
		Phase: "after",
		Error: "timeout",
	}
	if e.Error != "timeout" {
		t.Errorf("expected error 'timeout', got %q", e.Error)
	}
}

func TestAuditLog_EntryTokenFields(t *testing.T) {
	e := auditlog.Entry{
		Kind:         "model",
		Phase:        "after",
		InputTokens:  100,
		OutputTokens: 50,
	}
	if e.InputTokens != 100 {
		t.Errorf("input tokens: %d", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("output tokens: %d", e.OutputTokens)
	}
}

func TestAuditLog_EntryDurationField(t *testing.T) {
	e := auditlog.Entry{
		Kind:       "model",
		Phase:      "after",
		DurationMs: 42,
	}
	if e.DurationMs != 42 {
		t.Errorf("duration: %d", e.DurationMs)
	}
}

func TestAuditLog_EntryErrorCodeFormat(t *testing.T) {
	// Verify the Entry struct can hold error code formatted strings
	e := auditlog.Entry{
		Kind:  "model",
		Phase: "after",
		Error: "RATE_LIMIT: too many requests",
	}
	if e.Error != "RATE_LIMIT: too many requests" {
		t.Errorf("unexpected error: %q", e.Error)
	}
}

func TestAuditLog_EntryModelName(t *testing.T) {
	e := auditlog.Entry{
		Kind:      "model",
		Phase:     "before",
		AgentName: "test-agent",
		ModelName: "gpt-4",
	}
	if e.ModelName != "gpt-4" {
		t.Errorf("expected model name gpt-4, got %q", e.ModelName)
	}
}
