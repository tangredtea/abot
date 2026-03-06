package providers_test

import (
	"testing"

	"abot/pkg/providers"
	"abot/pkg/types"
)

// --- Registry tests ---

func TestRegistry_RegisterAndFindByName(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "test-provider", Keywords: []string{"test"}})

	got := r.FindByName("test-provider")
	if got == nil {
		t.Fatal("expected to find provider")
	}
	if got.Name != "test-provider" {
		t.Errorf("name: %q", got.Name)
	}
}

func TestRegistry_FindByName_Missing(t *testing.T) {
	r := providers.NewRegistry()
	if r.FindByName("nope") != nil {
		t.Error("expected nil for missing provider")
	}
}

func TestRegistry_FindByModel_KeywordMatch(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "anthropic", Keywords: []string{"claude"}})
	r.Register(types.ProviderSpec{Name: "openai", Keywords: []string{"gpt"}})

	got := r.FindByModel("claude-sonnet-4")
	if got == nil || got.Name != "anthropic" {
		t.Errorf("expected anthropic, got %v", got)
	}

	got = r.FindByModel("gpt-4o")
	if got == nil || got.Name != "openai" {
		t.Errorf("expected openai, got %v", got)
	}
}

func TestRegistry_FindByModel_PrefixMatch(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "anthropic", Keywords: []string{"claude"}})

	got := r.FindByModel("anthropic/claude-sonnet-4")
	if got == nil || got.Name != "anthropic" {
		t.Errorf("expected anthropic via prefix, got %v", got)
	}
}

func TestRegistry_FindByModel_SkipsGateway(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "openrouter", Keywords: []string{"openrouter"}, IsGateway: true})

	got := r.FindByModel("openrouter/gpt-4")
	if got != nil {
		t.Error("gateway should be skipped by FindByModel")
	}
}

func TestRegistry_FindByModel_NoMatch(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "anthropic", Keywords: []string{"claude"}})

	if r.FindByModel("llama-3") != nil {
		t.Error("expected nil for unmatched model")
	}
}

func TestRegistry_All(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(types.ProviderSpec{Name: "a"})
	r.Register(types.ProviderSpec{Name: "b"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	if all[0].Name != "a" || all[1].Name != "b" {
		t.Errorf("order: %q, %q", all[0].Name, all[1].Name)
	}
}

func TestDefaultRegistry_HasCommonProviders(t *testing.T) {
	r := providers.DefaultRegistry()
	for _, name := range []string{"anthropic", "openai", "deepseek", "gemini"} {
		if r.FindByName(name) == nil {
			t.Errorf("default registry missing %q", name)
		}
	}
}

// --- ParseModelRef tests (merged from both sources) ---

func TestParseModelRef_WithSlash(t *testing.T) {
	ref := providers.ParseModelRef("anthropic/claude-sonnet-4", "openai")
	if ref == nil {
		t.Fatal("expected non-nil")
	}
	if ref.Provider != "anthropic" || ref.Model != "claude-sonnet-4" {
		t.Errorf("got %+v", ref)
	}
}

func TestParseModelRef_NoSlash(t *testing.T) {
	ref := providers.ParseModelRef("gpt-4o", "openai")
	if ref == nil {
		t.Fatal("expected non-nil")
	}
	if ref.Provider != "openai" || ref.Model != "gpt-4o" {
		t.Errorf("got %+v", ref)
	}
}

func TestParseModelRef_Empty(t *testing.T) {
	if providers.ParseModelRef("", "openai") != nil {
		t.Error("expected nil for empty input")
	}
	if providers.ParseModelRef("openai/", "") != nil {
		t.Error("expected nil for trailing slash")
	}
}

// --- ParseModelRef table-driven (migrated from pkg/providers) ---

func TestParseModelRef_TableDriven(t *testing.T) {
	tests := []struct {
		raw       string
		defProv   string
		wantProv  string
		wantModel string
	}{
		{"anthropic/claude-3", "", "anthropic", "claude-3"},
		{"openai/gpt-4o", "", "openai", "gpt-4o"},
		{"claude-3-opus", "anthropic", "anthropic", "claude-3-opus"},
		{"gpt-4o", "openai", "openai", "gpt-4o"},
		{"gpt/gpt-4o", "", "openai", "gpt-4o"},       // normalized
		{"claude/sonnet", "", "anthropic", "sonnet"}, // normalized
	}
	for _, tt := range tests {
		ref := providers.ParseModelRef(tt.raw, tt.defProv)
		if ref == nil {
			t.Errorf("ParseModelRef(%q, %q) = nil", tt.raw, tt.defProv)
			continue
		}
		if ref.Provider != tt.wantProv {
			t.Errorf("ParseModelRef(%q).Provider = %q, want %q", tt.raw, ref.Provider, tt.wantProv)
		}
		if ref.Model != tt.wantModel {
			t.Errorf("ParseModelRef(%q).Model = %q, want %q", tt.raw, ref.Model, tt.wantModel)
		}
	}
}

// --- NormalizeProvider tests (merged) ---

func TestNormalizeProvider(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gpt", "openai"},
		{"claude", "anthropic"},
		{"google", "gemini"},
		{"deepseek", "deepseek"},
		{"  OpenAI  ", "openai"},
		{"openai", "openai"},
		{"DeepSeek", "deepseek"},
	}
	for _, c := range cases {
		if got := providers.NormalizeProvider(c.in); got != c.want {
			t.Errorf("NormalizeProvider(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- ModelKey tests (merged) ---

func TestModelKey(t *testing.T) {
	got := providers.ModelKey("Claude", "Claude-Sonnet-4")
	if got != "anthropic/claude-sonnet-4" {
		t.Errorf("got %q", got)
	}
}

func TestModelKey_GPT(t *testing.T) {
	got := providers.ModelKey("GPT", "GPT-4o")
	if got != "openai/gpt-4o" {
		t.Fatalf("ModelKey(GPT, GPT-4o) = %q, want openai/gpt-4o", got)
	}
}

// --- ContainsKeyword tests (migrated from pkg/providers) ---

func TestContainsKeyword(t *testing.T) {
	tests := []struct {
		name string
		s    string
		kw   string
		want bool
	}{
		{"exact match", "o1", "o1", true},
		{"prefix match", "o1-mini", "o1", true},
		{"suffix match", "model-o1", "o1", true},
		{"middle with separators", "my-o1-model", "o1", true},
		{"embedded in word", "pro1xy", "o1", false},
		{"embedded suffix", "foo3", "o3", false},
		{"embedded prefix", "o3bar", "o3", false},
		{"long keyword substring ok", "deepseek-chat", "deepseek", true},
		{"long keyword embedded ok", "mydeepseekmodel", "deepseek", true},
		{"slash separator", "openai/o1", "o1", true},
		{"underscore separator", "model_o3_mini", "o3", true},
		{"dot separator", "v2.o4.latest", "o4", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providers.ContainsKeyword(tt.s, tt.kw); got != tt.want {
				t.Errorf("ContainsKeyword(%q, %q) = %v, want %v", tt.s, tt.kw, got, tt.want)
			}
		})
	}
}

// --- FindByModel table-driven (migrated from pkg/providers) ---

func TestFindByModel_DefaultRegistry(t *testing.T) {
	r := providers.DefaultRegistry()

	tests := []struct {
		model    string
		wantName string
	}{
		{"claude-3-opus", "anthropic"},
		{"gpt-4o", "openai"},
		{"deepseek-chat", "deepseek"},
		{"gemini-pro", "gemini"},
		{"anthropic/claude-3", "anthropic"},
	}
	for _, tt := range tests {
		spec := r.FindByModel(tt.model)
		if spec == nil {
			t.Errorf("FindByModel(%q) = nil, want %s", tt.model, tt.wantName)
			continue
		}
		if spec.Name != tt.wantName {
			t.Errorf("FindByModel(%q).Name = %q, want %q", tt.model, spec.Name, tt.wantName)
		}
	}
}

func TestFindByModel_NoFalsePositive(t *testing.T) {
	r := providers.DefaultRegistry()

	// "pro1" should NOT match OpenAI's "o1" keyword.
	if spec := r.FindByModel("pro1"); spec != nil {
		t.Errorf("FindByModel(pro1) = %q, want nil", spec.Name)
	}
	// "foo3-model" should NOT match "o3".
	if spec := r.FindByModel("foo3-model"); spec != nil {
		t.Errorf("FindByModel(foo3-model) = %q, want nil", spec.Name)
	}
	// "o1-mini" SHOULD match OpenAI.
	if spec := r.FindByModel("o1-mini"); spec == nil || spec.Name != "openai" {
		name := ""
		if spec != nil {
			name = spec.Name
		}
		t.Errorf("FindByModel(o1-mini) = %q, want openai", name)
	}
}
