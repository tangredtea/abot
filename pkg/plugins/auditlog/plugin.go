package auditlog

import (
	"log/slog"
	"time"

	"google.golang.org/adk/plugin"
)

// Entry represents a single audit log record.
type Entry struct {
	Timestamp    time.Time      `json:"timestamp"`
	Kind         string         `json:"kind"`  // "model" or "tool"
	Phase        string         `json:"phase"` // "before" or "after"
	AgentName    string         `json:"agent_name"`
	ModelName    string         `json:"model_name,omitempty"`
	ToolName     string         `json:"tool_name,omitempty"`
	CallID       string         `json:"call_id,omitempty"`
	DurationMs   int64          `json:"duration_ms,omitempty"`
	InputTokens  int32          `json:"input_tokens,omitempty"`
	OutputTokens int32          `json:"output_tokens,omitempty"`
	Error        string         `json:"error,omitempty"`
	Args         map[string]any `json:"args,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
}

// Writer is the sink for audit entries.
type Writer interface {
	WriteEntry(entry Entry)
}

// Config holds the configuration for the audit log plugin.
type Config struct {
	Writer Writer // nil = default slog writer
}

// New creates an audit log plugin that records every LLM and tool call.
func New(cfg Config) (*plugin.Plugin, error) {
	w := cfg.Writer
	if w == nil {
		w = &slogWriter{logger: slog.Default()}
	}
	p := &AuditPlugin{
		writer:     w,
		modelStart: make(map[string]time.Time),
		toolStart:  make(map[string]time.Time),
	}
	return plugin.New(plugin.Config{
		Name:                "auditlog",
		BeforeModelCallback: p.BeforeModel,
		AfterModelCallback:  p.AfterModel,
		BeforeToolCallback:  p.beforeTool,
		AfterToolCallback:   p.afterTool,
	})
}

// slogWriter is the default Writer backed by slog.
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) WriteEntry(e Entry) {
	attrs := []any{
		slog.String("kind", e.Kind),
		slog.String("agent", e.AgentName),
		slog.String("phase", e.Phase),
	}
	if e.ModelName != "" {
		attrs = append(attrs, slog.String("model", e.ModelName))
	}
	if e.ToolName != "" {
		attrs = append(attrs, slog.String("tool", e.ToolName))
	}
	if e.DurationMs > 0 {
		attrs = append(attrs, slog.Int64("duration_ms", e.DurationMs))
	}
	if e.Error != "" {
		attrs = append(attrs, slog.String("error", e.Error))
	}
	w.logger.Info("audit", attrs...)
}
