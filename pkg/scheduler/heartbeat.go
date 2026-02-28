package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"abot/pkg/types"
)

// HeartbeatConfig holds configuration for the heartbeat service.
type HeartbeatConfig struct {
	Bus            types.MessageBus
	WorkspaceStore types.WorkspaceStore
	Tenants        types.TenantStore
	Interval       time.Duration // default 30m
	Channel        string        // target channel for heartbeat messages
	Logger         *slog.Logger
	LLM            model.LLM // LLM for decision making (can use SummaryLLM for cost savings)
	DecisionMode   string    // "passive" (default, backward compat) or "llm"
}

// HeartbeatService periodically checks each tenant's HEARTBEAT.md
// and publishes its content to the bus for agent processing.
type HeartbeatService struct {
	bus          types.MessageBus
	store        types.WorkspaceStore
	tenants      types.TenantStore
	interval     time.Duration
	channel      string
	mu           sync.Mutex // protects cancel
	cancel       context.CancelFunc
	done         chan struct{}
	logger       *slog.Logger
	llm          model.LLM
	decisionMode string
}

const heartbeatSystemPrompt = `You are a task scheduler. Review the following task list and decide if any tasks need to be executed right now based on their schedule and current time. Call the heartbeat_decision tool to report your decision.`

// heartbeatDecisionTool defines the structured tool for LLM heartbeat decisions.
func heartbeatDecisionTool() *genai.Tool {
	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "heartbeat_decision",
				Description: "Report whether scheduled tasks need execution right now",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"action": {
							Type:        genai.TypeString,
							Description: "skip = nothing to do, run = has active tasks",
							Enum:        []string{"skip", "run"},
						},
						"tasks": {
							Type:        genai.TypeString,
							Description: "Natural-language summary of tasks to execute (required when action is run)",
						},
					},
					Required: []string{"action"},
				},
			},
		},
	}
}

// NewHeartbeat creates a HeartbeatService from config.
func NewHeartbeat(cfg HeartbeatConfig) *HeartbeatService {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mode := cfg.DecisionMode
	if mode == "" {
		mode = "passive"
	}
	return &HeartbeatService{
		bus:          cfg.Bus,
		store:        cfg.WorkspaceStore,
		tenants:      cfg.Tenants,
		interval:     interval,
		channel:      cfg.Channel,
		done:         make(chan struct{}),
		logger:       logger,
		llm:          cfg.LLM,
		decisionMode: mode,
	}
}

// Start begins the periodic heartbeat loop.
func (h *HeartbeatService) Start(ctx context.Context) error {
	h.mu.Lock()
	ctx, h.cancel = context.WithCancel(ctx)
	h.mu.Unlock()
	go h.loop(ctx)
	h.logger.Info("heartbeat started", "interval", h.interval, "mode", h.decisionMode)
	return nil
}

// Stop cancels the heartbeat loop and waits for exit.
func (h *HeartbeatService) Stop() error {
	h.mu.Lock()
	cancel := h.cancel
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	<-h.done
	return nil
}

func (h *HeartbeatService) loop(ctx context.Context) {
	defer close(h.done)
	h.tick(ctx) // immediate first tick
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.tick(ctx)
		}
	}
}

// tick iterates all tenants, reads HEARTBEAT.md, optionally asks the LLM
// whether to act, and publishes actionable content to the bus.
func (h *HeartbeatService) tick(ctx context.Context) {
	tenants, err := h.tenants.List(ctx, "")
	if err != nil {
		h.logger.Error("heartbeat: list tenants", "err", err)
		return
	}

	for _, t := range tenants {
		if ctx.Err() != nil {
			return
		}
		doc, err := h.store.Get(ctx, t.TenantID, "HEARTBEAT")
		if err != nil || doc == nil || doc.Content == "" {
			continue
		}

		content := doc.Content
		meta := map[string]string{"source": "heartbeat"}

		if h.decisionMode == "llm" && h.llm != nil {
			action, tasks, llmErr := h.callLLMDecision(ctx, doc.Content)
			if llmErr != nil {
				h.logger.Error("heartbeat: llm decision", "tenant", t.TenantID, "err", llmErr)
				continue
			}
			if action != "run" {
				h.logger.Debug("heartbeat: llm decided to skip", "tenant", t.TenantID)
				continue
			}
			content = tasks
			meta["mode"] = "llm"
		}

		msg := types.InboundMessage{
			Channel:   h.channel,
			TenantID:  t.TenantID,
			Content:   content,
			Metadata:  meta,
			Timestamp: time.Now(),
		}
		if pubErr := h.bus.PublishInbound(ctx, msg); pubErr != nil {
			h.logger.Error("heartbeat: publish", "tenant", t.TenantID, "err", pubErr)
		}
	}
}

// callLLMDecision sends the heartbeat content to the LLM and parses its tool call decision.
func (h *HeartbeatService) callLLMDecision(ctx context.Context, content string) (action string, tasks string, err error) {
	if h.llm == nil {
		return "", "", fmt.Errorf("llm not configured for heartbeat decision mode")
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText(content, genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(heartbeatSystemPrompt, genai.RoleUser),
			Tools:             []*genai.Tool{heartbeatDecisionTool()},
		},
	}

	var lastResp *model.LLMResponse
	for resp, iterErr := range h.llm.GenerateContent(ctx, req, false) {
		if iterErr != nil {
			return "", "", fmt.Errorf("llm generate: %w", iterErr)
		}
		lastResp = resp
	}

	if lastResp == nil || lastResp.Content == nil {
		return "", "", fmt.Errorf("llm returned empty response")
	}

	for _, part := range lastResp.Content.Parts {
		if part.FunctionCall == nil || part.FunctionCall.Name != "heartbeat_decision" {
			continue
		}
		a, _ := part.FunctionCall.Args["action"].(string)
		t, _ := part.FunctionCall.Args["tasks"].(string)
		return a, t, nil
	}

	return "", "", fmt.Errorf("llm did not return heartbeat_decision tool call")
}
