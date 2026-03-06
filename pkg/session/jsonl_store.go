package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

// DefaultCompactThreshold is the file size (bytes) above which compaction is triggered.
const DefaultCompactThreshold = 10 * 1024 * 1024 // 10 MB

// DefaultCompactKeepEvents is the number of most recent events retained after compaction.
const DefaultCompactKeepEvents = 200

// JSONLService wraps the ADK in-memory session service with JSONL file persistence.
// On startup it loads events from disk and replays them into the in-memory delegate.
// On writes it persists to both in-memory and JSONL files (write-through).
type JSONLService struct {
	dir              string
	mu               sync.RWMutex
	inner            adksession.Service
	compactThreshold int64 // 0 means use DefaultCompactThreshold
	compactKeep      int   // 0 means use DefaultCompactKeepEvents
}

// metadataRecord is the first line written to each JSONL file.
type metadataRecord struct {
	Type      string         `json:"_type"`
	AppName   string         `json:"app_name"`
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	CreatedAt time.Time      `json:"created_at"`
	State     map[string]any `json:"state,omitempty"`
}

// eventRecord is the serialized form of an event written to JSONL.
type eventRecord struct {
	Type         string          `json:"_type"`
	ID           string          `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	InvocationID string          `json:"invocation_id,omitempty"`
	Author       string          `json:"author,omitempty"`
	Branch       string          `json:"branch,omitempty"`
	ContentJSON  json.RawMessage `json:"content,omitempty"`
	TurnComplete bool            `json:"turn_complete,omitempty"`
	Partial      bool            `json:"partial,omitempty"`
}

// Compile-time check that JSONLService implements adksession.Service.
var _ adksession.Service = (*JSONLService)(nil)

// NewJSONLService creates a new JSONL-backed session service rooted at dir.
// It loads any existing JSONL files from disk into the in-memory delegate.
func NewJSONLService(dir string) (*JSONLService, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("jsonl session: mkdir %s: %w", dir, err)
	}
	svc := &JSONLService{
		dir:   dir,
		inner: adksession.InMemoryService(),
	}
	if err := svc.loadAll(); err != nil {
		return nil, fmt.Errorf("jsonl session: load: %w", err)
	}
	return svc, nil
}

// sessionPath returns the JSONL file path for a given session.
func (s *JSONLService) sessionPath(appName, userID, sessionID string) string {
	return filepath.Join(s.dir, appName, userID+"_"+sessionID+".jsonl")
}

// Create delegates to the in-memory service and writes a metadata line to the JSONL file.
func (s *JSONLService) Create(ctx context.Context, req *adksession.CreateRequest) (*adksession.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp, err := s.inner.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("jsonl session: create: %w", err)
	}

	sess := resp.Session
	p := s.sessionPath(sess.AppName(), sess.UserID(), sess.ID())
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return nil, fmt.Errorf("jsonl session: mkdir: %w", err)
	}

	meta := metadataRecord{
		Type:      "metadata",
		AppName:   sess.AppName(),
		UserID:    sess.UserID(),
		SessionID: sess.ID(),
		CreatedAt: time.Now(),
		State:     req.State,
	}
	if err := s.appendLine(p, meta); err != nil {
		return nil, fmt.Errorf("jsonl session: write metadata: %w", err)
	}

	return resp, nil
}

// Get delegates to the in-memory service.
func (s *JSONLService) Get(ctx context.Context, req *adksession.GetRequest) (*adksession.GetResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inner.Get(ctx, req)
}

// List delegates to the in-memory service.
func (s *JSONLService) List(ctx context.Context, req *adksession.ListRequest) (*adksession.ListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inner.List(ctx, req)
}

// Delete delegates to the in-memory service and removes the JSONL file.
func (s *JSONLService) Delete(ctx context.Context, req *adksession.DeleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.inner.Delete(ctx, req); err != nil {
		return fmt.Errorf("jsonl session: delete: %w", err)
	}

	p := s.sessionPath(req.AppName, req.UserID, req.SessionID)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("jsonl session: remove file: %w", err)
	}
	return nil
}

// AppendEvent delegates to the in-memory service and appends the event to the JSONL file.
func (s *JSONLService) AppendEvent(ctx context.Context, sess adksession.Session, event *adksession.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.inner.AppendEvent(ctx, sess, event); err != nil {
		return fmt.Errorf("jsonl session: append event: %w", err)
	}

	// Partial events are skipped by the in-memory service, skip persistence too.
	if event.Partial {
		return nil
	}

	var contentJSON json.RawMessage
	if event.Content != nil {
		b, err := json.Marshal(event.Content)
		if err != nil {
			return fmt.Errorf("jsonl session: marshal content: %w", err)
		}
		contentJSON = b
	}

	rec := eventRecord{
		Type:         "event",
		ID:           event.ID,
		Timestamp:    event.Timestamp,
		InvocationID: event.InvocationID,
		Author:       event.Author,
		Branch:       event.Branch,
		ContentJSON:  contentJSON,
		TurnComplete: event.TurnComplete,
		Partial:      event.Partial,
	}

	p := s.sessionPath(sess.AppName(), sess.UserID(), sess.ID())
	if err := s.appendLine(p, rec); err != nil {
		return fmt.Errorf("jsonl session: persist event: %w", err)
	}
	s.maybeCompact(p)
	return nil
}

// appendLine marshals v as JSON and appends it as a single line to the file at path.
// Caller must hold s.mu.
func (s *JSONLService) appendLine(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("jsonl: marshal record: %w", err)
	}
	b = append(b, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("jsonl: open %s: %w", path, err)
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return fmt.Errorf("jsonl: write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("jsonl: sync %s: %w", path, err)
	}
	return f.Close()
}

// maybeCompact checks if the file exceeds the size threshold and compacts it.
// Called with s.mu already held.
func (s *JSONLService) maybeCompact(path string) {
	threshold := s.compactThreshold
	if threshold <= 0 {
		threshold = DefaultCompactThreshold
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < threshold {
		return
	}
	if err := s.compact(path); err != nil {
		// Best-effort: log and continue.
		slog.Warn("jsonl: compact failed", "path", path, "err", err)
	}
}

// compact rewrites a JSONL file keeping only the metadata line and the
// most recent N event lines (determined by compactKeep).
func (s *JSONLService) compact(path string) error {
	keep := s.compactKeep
	if keep <= 0 {
		keep = DefaultCompactKeepEvents
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("jsonl: read %s: %w", path, err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= keep+1 {
		return nil // nothing to compact
	}

	// First line is metadata, keep it. Then keep the last `keep` event lines.
	compacted := make([]string, 0, keep+1)
	compacted = append(compacted, lines[0])
	compacted = append(compacted, lines[len(lines)-keep:]...)

	tmp := path + ".compact.tmp"
	content := []byte(strings.Join(compacted, "\n") + "\n")
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("jsonl: create temp %s: %w", tmp, err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// loadAll scans the data directory for JSONL files and replays them into the in-memory service.
func (s *JSONLService) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("jsonl session: read dir %s: %w", s.dir, err)
	}

	ctx := context.Background()
	for _, appDir := range entries {
		if !appDir.IsDir() {
			continue
		}
		if err := s.loadAppDir(ctx, appDir.Name()); err != nil {
			return fmt.Errorf("load app %s: %w", appDir.Name(), err)
		}
	}
	return nil
}

// loadAppDir loads all JSONL files under a single app directory.
func (s *JSONLService) loadAppDir(ctx context.Context, appName string) error {
	dir := filepath.Join(s.dir, appName)
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("jsonl session: read app dir %s: %w", dir, err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}
		p := filepath.Join(dir, f.Name())
		if err := s.loadFile(ctx, p); err != nil {
			return fmt.Errorf("load %s: %w", p, err)
		}
	}
	return nil
}

// loadFile reads a single JSONL file and replays its metadata + events into the in-memory service.
func (s *JSONLService) loadFile(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("jsonl session: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// First line must be metadata.
	if !scanner.Scan() {
		return nil // empty file
	}

	var meta metadataRecord
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	if meta.Type != "metadata" {
		return fmt.Errorf("expected metadata record, got %q", meta.Type)
	}

	createResp, err := s.inner.Create(ctx, &adksession.CreateRequest{
		AppName:   meta.AppName,
		UserID:    meta.UserID,
		SessionID: meta.SessionID,
		State:     meta.State,
	})
	if err != nil {
		return fmt.Errorf("replay create: %w", err)
	}

	sess := createResp.Session
	return s.loadEvents(ctx, scanner, sess)
}

// loadEvents replays event lines from the scanner into the in-memory session.
func (s *JSONLService) loadEvents(ctx context.Context, scanner *bufio.Scanner, sess adksession.Session) error {
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec eventRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("parse event: %w", err)
		}
		if rec.Type != "event" {
			continue
		}

		event := &adksession.Event{
			ID:           rec.ID,
			Timestamp:    rec.Timestamp,
			InvocationID: rec.InvocationID,
			Author:       rec.Author,
			Branch:       rec.Branch,
		}
		event.TurnComplete = rec.TurnComplete
		event.Partial = rec.Partial

		// Deserialize Content from JSON (fixes content loss on restart).
		if len(rec.ContentJSON) > 0 && string(rec.ContentJSON) != "null" {
			var content genai.Content
			if err := json.Unmarshal(rec.ContentJSON, &content); err != nil {
				return fmt.Errorf("parse event content %s: %w", rec.ID, err)
			}
			event.Content = &content
		}

		if err := s.inner.AppendEvent(ctx, sess, event); err != nil {
			return fmt.Errorf("replay event %s: %w", rec.ID, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("jsonl session: scan events: %w", err)
	}
	return nil
}
