package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const (
	webSearchTimeout    = 15 * time.Second
	webFetchTimeout     = 15 * time.Second
	maxFetchBytes       = 50 * 1024  // 50KB
	maxSearchRespBytes  = 256 * 1024 // 256KB
	maxRedirects        = 5
	userAgent           = "Mozilla/5.0 (compatible; ABot/1.0)"
)

// privateNets contains CIDR ranges that web_fetch must never connect to.
// Covers loopback, RFC1918, link-local, and IPv6 equivalents.
var privateNets []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12",
		"192.168.0.0/16", "169.254.0.0/16", "0.0.0.0/8",
		"::1/128", "fc00::/7", "fe80::/10",
	} {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		privateNets = append(privateNets, n)
	}
}

// isPrivateIP returns true if ip falls within any blocked network range.
func isPrivateIP(ip net.IP) bool {
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeTransport returns an http.Transport whose DialContext resolves DNS
// first, then rejects connections to private/internal IP addresses.
func ssrfSafeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("SSRF blocked: %s resolves to private IP %s", host, ip.IP)
				}
			}
			// Connect to the first resolved address.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
}

// --- web_search ---

type webSearchArgs struct {
	Query      string `json:"query" jsonschema:"Search query"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"Max results to return (1-10, default 5)"`
}

type webSearchHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type webSearchResult struct {
	Results []webSearchHit `json:"results,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func newWebSearch(_ *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo. Returns titles, URLs, and snippets.",
	}, func(ctx tool.Context, args webSearchArgs) (webSearchResult, error) {
		limit := args.MaxResults
		if limit <= 0 || limit > 10 {
			limit = 5
		}
		hits, err := duckduckgoSearch(args.Query, limit)
		if err != nil {
			return webSearchResult{Error: err.Error()}, nil
		}
		return webSearchResult{Results: hits}, nil
	})
	return t
}

// ExtractText removes script/style tags and strips remaining HTML.
func ExtractText(html string) string {
	// Remove script and style blocks
	for _, tag := range []string{"script", "style"} {
		for {
			open := strings.Index(strings.ToLower(html), "<"+tag)
			if open < 0 {
				break
			}
			close := strings.Index(strings.ToLower(html[open:]), "</"+tag+">")
			if close < 0 {
				html = html[:open]
				break
			}
			html = html[:open] + html[open+close+len("</"+tag+">"):]
		}
	}
	text := StripTags(html)
	// Collapse whitespace
	lines := strings.Split(text, "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
// duckduckgoSearch queries DuckDuckGo HTML and extracts results.
func duckduckgoSearch(query string, limit int) ([]webSearchHit, error) {
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	client := &http.Client{Timeout: webSearchTimeout}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSearchRespBytes))
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return ParseDDGResults(string(body), limit), nil
}

// ParseDDGResults extracts search results from DuckDuckGo HTML.
func ParseDDGResults(html string, limit int) []webSearchHit {
	var hits []webSearchHit
	// Split on result class markers
	parts := strings.Split(html, "class=\"result__a\"")
	for i := 1; i < len(parts) && len(hits) < limit; i++ {
		hit := webSearchHit{}
		// Extract URL from href
		if idx := strings.Index(parts[i], "href=\""); idx >= 0 {
			rest := parts[i][idx+6:]
			if end := strings.Index(rest, "\""); end > 0 {
				rawURL := rest[:end]
				// DDG wraps URLs, try to extract the actual URL
				if u, err := url.QueryUnescape(rawURL); err == nil {
					rawURL = u
				}
				if strings.Contains(rawURL, "uddg=") {
					if parsed, err := url.Parse(rawURL); err == nil {
						if actual := parsed.Query().Get("uddg"); actual != "" {
							rawURL = actual
						}
					}
				}
				hit.URL = rawURL
			}
		}
		// Extract title from link text
		if idx := strings.Index(parts[i], ">"); idx >= 0 {
			rest := parts[i][idx+1:]
			if end := strings.Index(rest, "</a>"); end > 0 {
				hit.Title = StripTags(rest[:end])
			}
		}
		// Extract snippet
		if idx := strings.Index(parts[i], "class=\"result__snippet\""); idx >= 0 {
			rest := parts[i][idx:]
			if start := strings.Index(rest, ">"); start >= 0 {
				rest = rest[start+1:]
				if end := strings.Index(rest, "</"); end > 0 {
					hit.Snippet = StripTags(rest[:end])
				}
			}
		}
		if hit.URL != "" && hit.Title != "" {
			hits = append(hits, hit)
		}
	}
	return hits
}

// StripTags removes HTML tags from a string.
func StripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// --- web_fetch ---

type webFetchArgs struct {
	URL string `json:"url" jsonschema:"URL to fetch (http or https)"`
}

type webFetchResult struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
	Error       string `json:"error,omitempty"`
}

func newWebFetch(_ *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_fetch",
		Description: "Fetch a web page and return its text content. HTML tags are stripped.",
	}, func(ctx tool.Context, args webFetchArgs) (webFetchResult, error) {
		parsed, err := url.Parse(args.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return webFetchResult{Error: "invalid URL: must be http or https"}, nil
		}
		if parsed.Host == "" {
			return webFetchResult{Error: "invalid URL: missing host"}, nil
		}

		client := &http.Client{
			Timeout:   webFetchTimeout,
			Transport: ssrfSafeTransport(),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
		req, err := http.NewRequest("GET", args.URL, nil)
		if err != nil {
			return webFetchResult{Error: fmt.Sprintf("invalid request: %v", err)}, nil
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			return webFetchResult{Error: fmt.Sprintf("fetch failed: %v", err)}, nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxFetchBytes)+1))
		if err != nil {
			return webFetchResult{Error: fmt.Sprintf("read body failed: %v", err)}, nil
		}
		truncated := len(body) > maxFetchBytes
		if truncated {
			body = body[:maxFetchBytes]
		}

		ct := resp.Header.Get("Content-Type")
		text := string(body)

		// Strip HTML if content looks like HTML
		if strings.Contains(ct, "html") || strings.HasPrefix(strings.TrimSpace(text), "<") {
			text = ExtractText(text)
		}
		// Pretty-print JSON
		if strings.Contains(ct, "json") {
			var v any
			if json.Unmarshal(body, &v) == nil {
				if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
					text = string(pretty)
				}
			}
		}

		return webFetchResult{
			Status:      resp.StatusCode,
			ContentType: ct,
			Body:        text,
			Truncated:   truncated,
		}, nil
	})
	return t
}