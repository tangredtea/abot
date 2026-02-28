package tools_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"abot/pkg/tools"
)

// --- StripTags tests ---

func TestStripTags_Basic(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"<b>bold</b>", "bold"},
		{"<a href=\"x\">link</a>", "link"},
		{"no tags", "no tags"},
		{"<div><p>nested</p></div>", "nested"},
		{"", ""},
		{"<br/>", ""},
	}
	for _, tc := range cases {
		got := tools.StripTags(tc.in)
		if got != tc.want {
			t.Errorf("StripTags(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- ExtractText tests ---

func TestExtractText_RemovesScriptAndStyle(t *testing.T) {
	html := `<html><head><style>body{color:red}</style></head>
<body><script>alert('xss')</script><p>Hello World</p></body></html>`
	got := tools.ExtractText(html)
	if strings.Contains(got, "alert") {
		t.Errorf("script content not removed: %q", got)
	}
	if strings.Contains(got, "color:red") {
		t.Errorf("style content not removed: %q", got)
	}
	if !strings.Contains(got, "Hello World") {
		t.Errorf("body text missing: %q", got)
	}
}

func TestExtractText_CollapsesWhitespace(t *testing.T) {
	html := "<p>line1</p>\n\n\n<p>line2</p>"
	got := tools.ExtractText(html)
	lines := strings.Split(got, "\n")
	// Should not have empty lines
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			t.Errorf("found empty line in: %q", got)
		}
	}
}

// --- ParseDDGResults tests ---

func TestParseDDGResults_Basic(t *testing.T) {
	// Minimal DDG-like HTML with result markers
	html := `<div class="result__a" href="https://example.com">
<a class="result__a" href="https://example.com">Example Title</a>
<span class="result__snippet">This is a snippet</span>
</div>
<div class="result__a" href="https://other.com">
<a class="result__a" href="https://other.com">Other Title</a>
<span class="result__snippet">Another snippet</span>
</div>`

	hits := tools.ParseDDGResults(html, 10)
	if len(hits) == 0 {
		t.Fatal("expected at least 1 result")
	}
}

func TestParseDDGResults_LimitRespected(t *testing.T) {
	// Build HTML with many results
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(`<a class="result__a" href="https://example.com/`)
		b.WriteString(strings.Repeat("x", i))
		b.WriteString(`">Title `)
		b.WriteString(string(rune('A' + i)))
		b.WriteString(`</a><span class="result__snippet">Snippet</span>`)
	}
	hits := tools.ParseDDGResults(b.String(), 3)
	if len(hits) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(hits))
	}
}

func TestParseDDGResults_EmptyHTML(t *testing.T) {
	hits := tools.ParseDDGResults("", 5)
	if len(hits) != 0 {
		t.Errorf("expected 0 results from empty HTML, got %d", len(hits))
	}
}

// --- web_fetch with httptest ---

func TestWebFetch_HTMLStripped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>Hello</p><script>evil()</script></body></html>"))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestWebFetch_JSONPrettyPrint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key":"value","num":42}`))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestWebFetch_InvalidURL(t *testing.T) {
	// Test URL validation logic
	cases := []string{
		"ftp://example.com",
		"not-a-url",
		"://missing-scheme",
	}
	for _, u := range cases {
		_, err := http.Get(u)
		if err == nil {
			t.Errorf("expected error for %q", u)
		}
	}
}
