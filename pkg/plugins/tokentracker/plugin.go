package tokentracker

import (
	"sync/atomic"

	"google.golang.org/adk/plugin"
)

// Usage holds accumulated token counts (thread-safe).
type Usage struct {
	InputTokens  atomic.Int64
	OutputTokens atomic.Int64
	CallCount    atomic.Int64
}

// Snapshot returns a point-in-time copy of the counters.
func (u *Usage) Snapshot() UsageSnapshot {
	return UsageSnapshot{
		InputTokens:  u.InputTokens.Load(),
		OutputTokens: u.OutputTokens.Load(),
		CallCount:    u.CallCount.Load(),
	}
}

// UsageSnapshot is a serializable copy of Usage.
type UsageSnapshot struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CallCount    int64 `json:"call_count"`
}

// Callback is invoked after each model call with the delta.
type Callback func(agentName string, input, output int32)

// Config for the token tracker plugin.
type Config struct {
	// OnRecord is called after each model call. Optional.
	OnRecord Callback
}

// New creates a token tracking plugin.
// Access accumulated usage via Tracker.Global() or Tracker.ByAgent().
func New(cfg Config) (*plugin.Plugin, *Tracker, error) {
	t := &Tracker{
		global:   &Usage{},
		byAgent:  make(map[string]*Usage),
		onRecord: cfg.OnRecord,
	}
	p, err := plugin.New(plugin.Config{
		Name:               "tokentracker",
		AfterModelCallback: t.AfterModel,
	})
	if err != nil {
		return nil, nil, err
	}
	return p, t, nil
}
