package providers_test

import (
	"testing"

	"abot/pkg/providers"
)

// --- ExtractProtocol tests ---

func TestExtractProtocol_WithPrefix(t *testing.T) {
	cases := []struct {
		in       string
		protocol string
		model    string
	}{
		{"anthropic/claude-sonnet-4", "anthropic", "claude-sonnet-4"},
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{"deepseek/deepseek-chat", "deepseek", "deepseek-chat"},
	}
	for _, c := range cases {
		p, m := providers.ExtractProtocol(c.in)
		if p != c.protocol || m != c.model {
			t.Errorf("ExtractProtocol(%q) = (%q, %q), want (%q, %q)",
				c.in, p, m, c.protocol, c.model)
		}
	}
}

func TestExtractProtocol_NoPrefix(t *testing.T) {
	p, m := providers.ExtractProtocol("gpt-4o")
	if p != "openai" {
		t.Errorf("protocol: %q, want openai", p)
	}
	if m != "gpt-4o" {
		t.Errorf("model: %q", m)
	}
}

// --- ExtractProtocol table-driven (migrated from pkg/providers) ---

func TestExtractProtocol_TableDriven(t *testing.T) {
	tests := []struct {
		input       string
		wantProto   string
		wantModelID string
	}{
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{"anthropic/claude-sonnet-4.6", "anthropic", "claude-sonnet-4.6"},
		{"gpt-4o", "openai", "gpt-4o"},
		{"deepseek/deepseek-chat", "deepseek", "deepseek-chat"},
		{"ollama/llama3", "ollama", "llama3"},
		{"  gemini/gemini-pro  ", "gemini", "gemini-pro"},
	}
	for _, tt := range tests {
		proto, modelID := providers.ExtractProtocol(tt.input)
		if proto != tt.wantProto {
			t.Errorf("ExtractProtocol(%q) proto = %q, want %q", tt.input, proto, tt.wantProto)
		}
		if modelID != tt.wantModelID {
			t.Errorf("ExtractProtocol(%q) modelID = %q, want %q", tt.input, modelID, tt.wantModelID)
		}
	}
}

// --- GetDefaultAPIBase tests (migrated from pkg/providers) ---

func TestGetDefaultAPIBase(t *testing.T) {
	tests := []struct {
		protocol string
		wantBase string
	}{
		{"openai", "https://api.openai.com/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"ollama", "http://localhost:11434/v1"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := providers.GetDefaultAPIBase(tt.protocol)
		if got != tt.wantBase {
			t.Errorf("GetDefaultAPIBase(%q) = %q, want %q", tt.protocol, got, tt.wantBase)
		}
	}
}

// --- CreateModelFromConfig tests (merged) ---

func TestCreateModelFromConfig_EmptyModel(t *testing.T) {
	_, _, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestCreateModelFromConfig_Anthropic(t *testing.T) {
	m, id, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
		Model:  "anthropic/claude-sonnet-4",
		APIKey: "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "claude-sonnet-4" {
		t.Errorf("model id: %q", id)
	}
	if m.Name() != "claude-sonnet-4" {
		t.Errorf("model name: %q", m.Name())
	}
}

func TestCreateModelFromConfig_Anthropic_NoKey(t *testing.T) {
	_, _, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
		Model: "anthropic/claude-sonnet-4",
	})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

func TestCreateModelFromConfig_OpenAI(t *testing.T) {
	m, id, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
		Model:  "openai/gpt-4o",
		APIKey: "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "gpt-4o" {
		t.Errorf("model id: %q", id)
	}
	if m.Name() != "gpt-4o" {
		t.Errorf("model name: %q", m.Name())
	}
}

func TestCreateModelFromConfig_DeepSeek(t *testing.T) {
	m, _, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
		Model:  "deepseek/deepseek-chat",
		APIKey: "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Name() != "deepseek-chat" {
		t.Errorf("model name: %q", m.Name())
	}
}

// --- CreateModelFromConfig_NoPrefix (migrated from pkg/providers) ---

func TestCreateModelFromConfig_NoPrefix(t *testing.T) {
	m, modelID, err := providers.CreateModelFromConfig(providers.ModelFactoryConfig{
		Model:  "gpt-4o",
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modelID != "gpt-4o" {
		t.Errorf("modelID = %q, want %q", modelID, "gpt-4o")
	}
	if m == nil {
		t.Fatal("expected non-nil LLM")
	}
}
