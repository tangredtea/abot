package storage_test

import (
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"

	"abot/pkg/storage/vectordb"
)

func TestCollectionName(t *testing.T) {
	if got := vectordb.CollectionName("tenant-abc"); got != "tenant_tenant-abc" {
		t.Errorf("got %q", got)
	}
}

func TestExtractEventText_WithText(t *testing.T) {
	ev := &session.Event{
		LLMResponse: model.LLMResponse{
			Content: &genai.Content{
				Parts: []*genai.Part{
					{Text: "hello"},
					{Text: "world"},
				},
			},
		},
	}
	got := vectordb.ExtractEventText(ev)
	if got != "hello\nworld" {
		t.Errorf("got %q", got)
	}
}

func TestExtractEventText_NilContent(t *testing.T) {
	ev := &session.Event{}
	if got := vectordb.ExtractEventText(ev); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestNewMemoryService(t *testing.T) {
	ms := vectordb.NewMemoryService(nil, nil)
	if ms == nil {
		t.Error("expected non-nil MemoryService")
	}
}
