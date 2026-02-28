package tokentracker

import (
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
)

// Tracker accumulates token usage globally and per-agent.
type Tracker struct {
	global   *Usage
	mu       sync.RWMutex
	byAgent  map[string]*Usage
	onRecord Callback
}

// Global returns the aggregate usage across all agents.
func (t *Tracker) Global() *Usage { return t.global }

// ByAgent returns usage for a specific agent. Returns nil if unseen.
func (t *Tracker) ByAgent(name string) *Usage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.byAgent[name]
}

// AllAgents returns a snapshot map of agent name → usage snapshot.
func (t *Tracker) AllAgents() map[string]UsageSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]UsageSnapshot, len(t.byAgent))
	for k, v := range t.byAgent {
		out[k] = v.Snapshot()
	}
	return out
}

func (t *Tracker) getOrCreateAgent(name string) *Usage {
	t.mu.RLock()
	u, ok := t.byAgent[name]
	t.mu.RUnlock()
	if ok {
		return u
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if u, ok = t.byAgent[name]; ok {
		return u
	}
	u = &Usage{}
	t.byAgent[name] = u
	return u
}

func (t *Tracker) AfterModel(ctx agent.CallbackContext, resp *model.LLMResponse, _ error) (*model.LLMResponse, error) {
	if resp == nil || resp.UsageMetadata == nil {
		return nil, nil
	}
	in := resp.UsageMetadata.PromptTokenCount
	out := resp.UsageMetadata.CandidatesTokenCount

	t.global.InputTokens.Add(int64(in))
	t.global.OutputTokens.Add(int64(out))
	t.global.CallCount.Add(1)

	agentName := ctx.AgentName()
	au := t.getOrCreateAgent(agentName)
	au.InputTokens.Add(int64(in))
	au.OutputTokens.Add(int64(out))
	au.CallCount.Add(1)

	if t.onRecord != nil {
		t.onRecord(agentName, in, out)
	}
	return nil, nil
}
