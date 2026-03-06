package agent_test

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"testing"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	"abot/pkg/types"
)

// mockLLM implements model.LLM for testing.
type mockLLM struct {
	name     string
	response string
	err      error
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if m.err != nil {
			yield(nil, m.err)
			return
		}
		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: m.response}},
			},
		}, nil)
	}
}

// mockBus implements types.MessageBus for testing.
type mockBus struct {
	mu       sync.Mutex
	inbound  []types.InboundMessage
	outbound []types.OutboundMessage
	closed   bool
	inCh     chan types.InboundMessage
}

func newMockBus() *mockBus {
	return &mockBus{
		inCh: make(chan types.InboundMessage, 10),
	}
}

func (b *mockBus) PublishInbound(_ context.Context, msg types.InboundMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inbound = append(b.inbound, msg)
	select {
	case b.inCh <- msg:
	default:
	}
	return nil
}

func (b *mockBus) ConsumeInbound(ctx context.Context) (types.InboundMessage, error) {
	select {
	case msg := <-b.inCh:
		return msg, nil
	case <-ctx.Done():
		return types.InboundMessage{}, ctx.Err()
	}
}

func (b *mockBus) PublishOutbound(_ context.Context, msg types.OutboundMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.outbound = append(b.outbound, msg)
	return nil
}

func (b *mockBus) ConsumeOutbound(ctx context.Context) (types.OutboundMessage, error) {
	return types.OutboundMessage{}, fmt.Errorf("not implemented")
}

func (b *mockBus) InboundSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.inbound)
}

func (b *mockBus) OutboundSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.outbound)
}

func (b *mockBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *mockBus) getOutbound() []types.OutboundMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]types.OutboundMessage, len(b.outbound))
	copy(out, b.outbound)
	return out
}

// compile-time checks
var (
	_ model.LLM        = (*mockLLM)(nil)
	_ types.MessageBus = (*mockBus)(nil)
)

// newTestRunner creates a runner with a custom agent that echoes back a fixed response.
func newTestRunner(t *testing.T, ss session.Service, appName, agentName, response string) (*runner.Runner, adkagent.Agent) {
	t.Helper()

	a, err := adkagent.New(adkagent.Config{
		Name:        agentName,
		Description: "test agent",
		Run: func(ctx adkagent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				ev := session.NewEvent(ctx.InvocationID())
				ev.Author = agentName
				ev.LLMResponse = model.LLMResponse{
					Content: &genai.Content{
						Role:  "model",
						Parts: []*genai.Part{{Text: response}},
					},
					TurnComplete: true,
				}
				yield(ev, nil)
			}
		},
	})
	if err != nil {
		t.Fatalf("create test agent: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          a,
		SessionService: ss,
	})
	if err != nil {
		t.Fatalf("create test runner: %v", err)
	}
	return r, a
}

// newTestRunnerWithLLM creates a runner using the specified LLM, for testing error scenarios.
func newTestRunnerWithLLM(t *testing.T, ss session.Service, appName, agentName string, llm model.LLM) (*runner.Runner, adkagent.Agent) {
	t.Helper()

	a, err := adkagent.New(adkagent.Config{
		Name:        agentName,
		Description: "test agent with custom LLM",
		Run: func(ctx adkagent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				req := &model.LLMRequest{
					Contents: []*genai.Content{
						{Role: "user", Parts: []*genai.Part{{Text: "test"}}},
					},
				}
				for resp, err := range llm.GenerateContent(ctx, req, false) {
					if err != nil {
						yield(nil, err)
						return
					}
					ev := session.NewEvent(ctx.InvocationID())
					ev.Author = agentName
					ev.LLMResponse = model.LLMResponse{
						Content:      resp.Content,
						TurnComplete: true,
					}
					yield(ev, nil)
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("create test agent: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          a,
		SessionService: ss,
	})
	if err != nil {
		t.Fatalf("create test runner: %v", err)
	}
	return r, a
}
