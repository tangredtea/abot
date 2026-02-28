package auditlog

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

type AuditPlugin struct {
	writer     Writer
	mu         sync.Mutex
	modelStart map[string]time.Time // invocationID:agent → start
	toolStart  map[string]time.Time // functionCallID → start
}

func modelTimerKey(ctx agent.CallbackContext) string {
	return fmt.Sprintf("%s:%s", ctx.InvocationID(), ctx.AgentName())
}

func (p *AuditPlugin) BeforeModel(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	key := modelTimerKey(ctx)
	p.mu.Lock()
	p.modelStart[key] = time.Now()
	p.mu.Unlock()

	e := Entry{
		Timestamp: time.Now(),
		Kind:      "model",
		Phase:     "before",
		AgentName: ctx.AgentName(),
	}
	if req != nil && req.Model != "" {
		e.ModelName = req.Model
	}
	p.writer.WriteEntry(e)
	return nil, nil
}

func (p *AuditPlugin) AfterModel(ctx agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
	key := modelTimerKey(ctx)
	p.mu.Lock()
	start, ok := p.modelStart[key]
	if ok {
		delete(p.modelStart, key)
	}
	p.mu.Unlock()

	e := Entry{
		Timestamp: time.Now(),
		Kind:      "model",
		Phase:     "after",
		AgentName: ctx.AgentName(),
	}
	if ok {
		e.DurationMs = time.Since(start).Milliseconds()
	}
	if respErr != nil {
		e.Error = respErr.Error()
	}
	if resp != nil {
		if resp.UsageMetadata != nil {
			e.InputTokens = resp.UsageMetadata.PromptTokenCount
			e.OutputTokens = resp.UsageMetadata.CandidatesTokenCount
		}
		if resp.ErrorCode != "" {
			e.Error = fmt.Sprintf("%s: %s", resp.ErrorCode, resp.ErrorMessage)
		}
	}
	p.writer.WriteEntry(e)
	return nil, nil
}

func (p *AuditPlugin) beforeTool(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	callID := ctx.FunctionCallID()
	p.mu.Lock()
	p.toolStart[callID] = time.Now()
	p.mu.Unlock()

	p.writer.WriteEntry(Entry{
		Timestamp: time.Now(),
		Kind:      "tool",
		Phase:     "before",
		AgentName: ctx.AgentName(),
		ToolName:  t.Name(),
		CallID:    callID,
		Args:      args,
	})
	return nil, nil
}

func (p *AuditPlugin) afterTool(ctx tool.Context, t tool.Tool, args, result map[string]any, toolErr error) (map[string]any, error) {
	callID := ctx.FunctionCallID()
	p.mu.Lock()
	start, ok := p.toolStart[callID]
	if ok {
		delete(p.toolStart, callID)
	}
	p.mu.Unlock()

	e := Entry{
		Timestamp: time.Now(),
		Kind:      "tool",
		Phase:     "after",
		AgentName: ctx.AgentName(),
		ToolName:  t.Name(),
		CallID:    callID,
		Result:    result,
	}
	if ok {
		e.DurationMs = time.Since(start).Milliseconds()
	}
	if toolErr != nil {
		e.Error = toolErr.Error()
	}
	p.writer.WriteEntry(e)
	return nil, nil
}
