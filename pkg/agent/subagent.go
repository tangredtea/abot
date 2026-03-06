package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"abot/pkg/types"
)

// SubagentTask tracks a spawned background task.
type SubagentTask struct {
	ID            string
	Task          string
	AgentID       string
	OriginChannel string
	OriginChatID  string
	TenantID      string
	UserID        string
	Status        string // "running", "completed", "failed"
	Result        string
}

// SubagentManager spawns and tracks background agent tasks.
type SubagentManager struct {
	registry       *AgentRegistry
	bus            types.MessageBus
	sessionService session.Service
	appName        string
	tasks          map[string]*SubagentTask
	mu             sync.RWMutex
	wg             sync.WaitGroup
}

// NewSubagentManager creates a new manager.
func NewSubagentManager(reg *AgentRegistry, bus types.MessageBus, ss session.Service, appName string) *SubagentManager {
	return &SubagentManager{
		registry:       reg,
		bus:            bus,
		sessionService: ss,
		appName:        appName,
		tasks:          make(map[string]*SubagentTask),
	}
}

// Spawn launches a background task on the target agent.
func (sm *SubagentManager) Spawn(ctx context.Context, task, agentID, channel, chatID, tenantID, userID string) (string, error) {
	r, ok := sm.registry.GetRunner(agentID)
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentID)
	}

	sm.mu.Lock()
	taskID := "subtask-" + uuid.NewString()[:8]
	st := &SubagentTask{
		ID:            taskID,
		Task:          task,
		AgentID:       agentID,
		OriginChannel: channel,
		OriginChatID:  chatID,
		TenantID:      tenantID,
		UserID:        userID,
		Status:        "running",
	}
	sm.tasks[taskID] = st
	sm.mu.Unlock()

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()
		sm.execute(ctx, r, st)
	}()
	return taskID, nil
}

func (sm *SubagentManager) execute(ctx context.Context, r *runner.Runner, st *SubagentTask) {
	// Create a dedicated session for this subtask with tenant context propagated.
	sessionID := "spawn-" + st.ID
	userID := st.UserID
	if userID == "" {
		userID = "system"
	}

	_, err := sm.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   sm.appName,
		UserID:    userID,
		SessionID: sessionID,
		State: map[string]any{
			"tenant_id": st.TenantID,
			"user_id":   userID,
			"channel":   st.OriginChannel,
			"chat_id":   st.OriginChatID,
		},
	})
	if err != nil {
		sm.finishTask(st, "failed", fmt.Sprintf("create session: %v", err))
		return
	}

	msg := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: st.Task}},
	}

	var result string
	for ev, err := range r.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			sm.finishTask(st, "failed", fmt.Sprintf("run error: %v", err))
			return
		}
		if ev != nil && ev.IsFinalResponse() && ev.Content != nil {
			for _, p := range ev.Content.Parts {
				if p.Text != "" {
					result += p.Text
				}
			}
		}
	}

	sm.finishTask(st, "completed", result)

	// Notify origin channel.
	_ = sm.bus.PublishOutbound(ctx, types.OutboundMessage{
		Channel: st.OriginChannel,
		ChatID:  st.OriginChatID,
		Content: fmt.Sprintf("[Subtask %s completed]\n%s", st.ID, result),
	})
}

func (sm *SubagentManager) finishTask(st *SubagentTask, status, result string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	st.Status = status
	st.Result = result
}

// GetTask returns a subtask by ID.
func (sm *SubagentManager) GetTask(taskID string) (*SubagentTask, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	t, ok := sm.tasks[taskID]
	return t, ok
}

// Wait blocks until all spawned goroutines have finished.
func (sm *SubagentManager) Wait() {
	sm.wg.Wait()
}

// ---------------------------------------------------------------------------
// The following methods implement the tools.SubagentSpawner interface
// (satisfied implicitly — no need to import the tools package).
// ---------------------------------------------------------------------------

// SpawnSync executes a sub-agent task synchronously, blocking until completion.
func (sm *SubagentManager) SpawnSync(ctx context.Context, task, agentID, channel, chatID, tenantID, userID string) (string, error) {
	r, ok := sm.registry.GetRunner(agentID)
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentID)
	}

	sessionID := "sync-" + uuid.NewString()[:8]
	if userID == "" {
		userID = "system"
	}

	_, err := sm.sessionService.Create(ctx, &session.CreateRequest{
		AppName:   sm.appName,
		UserID:    userID,
		SessionID: sessionID,
		State: map[string]any{
			"tenant_id": tenantID,
			"user_id":   userID,
			"channel":   channel,
			"chat_id":   chatID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	msg := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: task}},
	}

	var sb strings.Builder
	for ev, err := range r.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			return "", fmt.Errorf("run error: %w", err)
		}
		if ev != nil && ev.IsFinalResponse() && ev.Content != nil {
			for _, p := range ev.Content.Parts {
				if p.Text != "" {
					sb.WriteString(p.Text)
				}
			}
		}
	}
	return sb.String(), nil
}

// SpawnAsync starts a sub-agent task asynchronously and returns the task ID.
// Wraps the existing Spawn method.
func (sm *SubagentManager) SpawnAsync(ctx context.Context, task, agentID, channel, chatID, tenantID, userID string) (string, error) {
	return sm.Spawn(ctx, task, agentID, channel, chatID, tenantID, userID)
}

// GetTaskStatus returns the current status and result of a sub-task.
func (sm *SubagentManager) GetTaskStatus(taskID string) (status, result string, found bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	t, ok := sm.tasks[taskID]
	if !ok {
		return "", "", false
	}
	return t.Status, t.Result, true
}

// ListTasks returns summaries of all sub-tasks.
// Returns []types.TaskSummary, implicitly satisfying tools.SubagentSpawner.
func (sm *SubagentManager) ListTasks() []types.TaskSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	out := make([]types.TaskSummary, 0, len(sm.tasks))
	for _, t := range sm.tasks {
		out = append(out, types.TaskSummary{
			ID:      t.ID,
			AgentID: t.AgentID,
			Status:  t.Status,
			Task:    t.Task,
		})
	}
	return out
}
