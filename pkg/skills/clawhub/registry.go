package clawhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"abot/pkg/skills"
	"abot/pkg/types"
)

const (
	defaultTimeout         = 30 * time.Second
	DefaultMaxZipSize      = 50 * 1024 * 1024 // 50 MB
	defaultMaxResponseSize = 2 * 1024 * 1024  // 2 MB
)

var SlugPattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// Registry implements skills.SkillRegistry for the ClawHub platform.
type Registry struct {
	baseURL         string
	authToken       string
	searchPath      string
	skillsPath      string
	downloadPath    string
	maxZipSize      int
	maxResponseSize int
	client          *http.Client
}

// New creates a ClawHub registry client from config.
func New(cfg skills.ClawHubConfig) *Registry {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://clawhub.ai"
	}
	searchPath := cfg.SearchPath
	if searchPath == "" {
		searchPath = "/api/v1/search"
	}
	skillsPath := cfg.SkillsPath
	if skillsPath == "" {
		skillsPath = "/api/v1/skills"
	}
	downloadPath := cfg.DownloadPath
	if downloadPath == "" {
		downloadPath = "/api/v1/download"
	}

	timeout := defaultTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}
	maxZip := DefaultMaxZipSize
	if cfg.MaxZipSize > 0 {
		maxZip = cfg.MaxZipSize
	}
	maxResp := defaultMaxResponseSize
	if cfg.MaxResponseSize > 0 {
		maxResp = cfg.MaxResponseSize
	}

	return &Registry{
		baseURL:         baseURL,
		authToken:       cfg.AuthToken,
		searchPath:      searchPath,
		skillsPath:      skillsPath,
		downloadPath:    downloadPath,
		maxZipSize:      maxZip,
		maxResponseSize: maxResp,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

func (r *Registry) Name() string { return "clawhub" }

// --- Search ---

type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Score       float64 `json:"score"`
	Slug        *string `json:"slug"`
	DisplayName *string `json:"displayName"`
	Summary     *string `json:"summary"`
	Version     *string `json:"version"`
}

func (r *Registry) Search(ctx context.Context, query string, limit int) ([]skills.SearchResult, error) {
	u, err := url.Parse(r.baseURL + r.searchPath)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	q := u.Query()
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = q.Encode()

	body, err := r.doGet(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	results := make([]skills.SearchResult, 0, len(resp.Results))
	for _, sr := range resp.Results {
		slug := Deref(sr.Slug)
		if slug == "" {
			continue
		}
		summary := Deref(sr.Summary)
		if summary == "" {
			continue
		}
		displayName := Deref(sr.DisplayName)
		if displayName == "" {
			displayName = slug
		}
		results = append(results, skills.SearchResult{
			Score:        sr.Score,
			Slug:         slug,
			DisplayName:  displayName,
			Summary:      summary,
			Version:      Deref(sr.Version),
			RegistryName: r.Name(),
		})
	}
	return results, nil
}

// --- GetSkillMeta ---

type skillResponse struct {
	Slug          string          `json:"slug"`
	DisplayName   string          `json:"displayName"`
	Summary       string          `json:"summary"`
	LatestVersion *versionInfo    `json:"latestVersion"`
	Moderation    *moderationInfo `json:"moderation"`
}

type versionInfo struct {
	Version string `json:"version"`
}

type moderationInfo struct {
	IsMalwareBlocked bool `json:"isMalwareBlocked"`
	IsSuspicious     bool `json:"isSuspicious"`
}

func (r *Registry) GetSkillMeta(ctx context.Context, slug string) (*skills.SkillMeta, error) {
	if !SlugPattern.MatchString(slug) {
		return nil, fmt.Errorf("invalid slug %q", slug)
	}

	u := r.baseURL + r.skillsPath + "/" + url.PathEscape(slug)
	body, err := r.doGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("skill metadata request failed: %w", err)
	}

	var resp skillResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse skill metadata: %w", err)
	}

	meta := &skills.SkillMeta{
		Slug:         resp.Slug,
		DisplayName:  resp.DisplayName,
		Summary:      resp.Summary,
		RegistryName: r.Name(),
	}
	if resp.LatestVersion != nil {
		meta.LatestVersion = resp.LatestVersion.Version
	}
	if resp.Moderation != nil {
		meta.IsMalwareBlocked = resp.Moderation.IsMalwareBlocked
		meta.IsSuspicious = resp.Moderation.IsSuspicious
	}
	return meta, nil
}

// --- DownloadAndInstall ---

// DownloadAndInstall fetches metadata, resolves version, downloads the skill
// package, and uploads it to the object store at objectPath.
func (r *Registry) DownloadAndInstall(
	ctx context.Context,
	slug, version string,
	objStore types.ObjectStore,
	objectPath string,
) (*skills.InstallResult, error) {
	if !SlugPattern.MatchString(slug) {
		return nil, fmt.Errorf("invalid slug %q", slug)
	}

	result := &skills.InstallResult{}

	// Step 1: Fetch metadata (fallback on error).
	meta, err := r.GetSkillMeta(ctx, slug)
	if err != nil {
		meta = nil
	}
	if meta != nil {
		result.IsMalwareBlocked = meta.IsMalwareBlocked
		result.IsSuspicious = meta.IsSuspicious
		result.Summary = meta.Summary
	}

	// Step 2: Resolve version.
	installVersion := version
	if installVersion == "" && meta != nil {
		installVersion = meta.LatestVersion
	}
	if installVersion == "" {
		installVersion = "latest"
	}
	result.Version = installVersion

	// Step 3: Download package.
	u, err := url.Parse(r.baseURL + r.downloadPath)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	q := u.Query()
	q.Set("slug", slug)
	if installVersion != "latest" {
		q.Set("version", installVersion)
	}
	u.RawQuery = q.Encode()

	data, err := r.doGetRaw(ctx, u.String(), int64(r.maxZipSize))
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Step 4: Upload to object store.
	if err := objStore.Put(ctx, objectPath, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("upload to object store failed: %w", err)
	}

	return result, nil
}

// --- HTTP helpers ---

func (r *Registry) doGet(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("clawhub: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub: request %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(r.maxResponseSize)))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (r *Registry) doGetRaw(ctx context.Context, urlStr string, maxSize int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("clawhub: create download request: %w", err)
	}
	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub: download %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return data, nil
}

func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
