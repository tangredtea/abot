package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
)

// Model implements model.LLM for OpenAI-compatible ChatCompletion APIs.
type Model struct {
	name    string
	apiKey  string
	apiBase string
	client  *http.Client
}

// Config holds OpenAI-compatible model configuration.
type Config struct {
	Name    string // model name, e.g. "gpt-4o"
	APIKey  string
	APIBase string // e.g. "https://api.openai.com/v1"
	Client  *http.Client
}

// NewModel creates an OpenAI-compatible model.LLM implementation.
func NewModel(cfg Config) *Model {
	base := strings.TrimRight(cfg.APIBase, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &Model{
		name:    cfg.Name,
		apiKey:  cfg.APIKey,
		apiBase: base,
		client:  client,
	}
}

func (m *Model) Name() string { return m.name }

// GenerateContent implements model.LLM.
func (m *Model) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return m.generateStream(ctx, req)
	}
	return func(yield func(*model.LLMResponse, error) bool) {
		resp, err := m.generate(ctx, req)
		yield(resp, err)
	}
}

func (m *Model) endpoint() string {
	return m.apiBase + "/chat/completions"
}

func (m *Model) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
	}
}

// generate performs a synchronous ChatCompletion call.
func (m *Model) generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	apiReq, err := ConvertRequest(req, m.name, false)
	if err != nil {
		return nil, fmt.Errorf("openai: convert request: %w", err)
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	slog.Debug("openai: request", "model", m.name, "endpoint", m.endpoint(), "stream", false, "body_len", len(body), "tools", len(apiReq.Tools))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	m.setHeaders(httpReq)

	start := time.Now()
	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		slog.Error("openai: HTTP error", "model", m.name, "elapsed", time.Since(start), "err", err)
		return nil, fmt.Errorf("openai: HTTP %s: %w", m.name, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}

	slog.Debug("openai: response", "model", m.name, "status", httpResp.StatusCode, "elapsed", time.Since(start), "body_len", len(respBody))

	if httpResp.StatusCode != http.StatusOK {
		var apiErr ChatError
		_ = json.Unmarshal(respBody, &apiErr)
		slog.Error("openai: API error", "model", m.name, "status", httpResp.StatusCode, "type", apiErr.Error.Type, "message", apiErr.Error.Message)
		return nil, fmt.Errorf("openai: HTTP %d: %s: %s",
			httpResp.StatusCode, apiErr.Error.Type, apiErr.Error.Message)
	}

	var resp ChatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	// Log tool calls if any.
	if len(resp.Choices) > 0 && len(resp.Choices[0].Message.ToolCalls) > 0 {
		names := make([]string, len(resp.Choices[0].Message.ToolCalls))
		for i, tc := range resp.Choices[0].Message.ToolCalls {
			names[i] = tc.Function.Name
		}
		slog.Debug("openai: tool_calls", "model", m.name, "tools", names)
	}

	return ConvertResponse(&resp), nil
}

// generateStream performs a streaming ChatCompletion call.
func (m *Model) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		apiReq, err := ConvertRequest(req, m.name, true)
		if err != nil {
			yield(nil, fmt.Errorf("openai: convert request: %w", err))
			return
		}

		body, err := json.Marshal(apiReq)
		if err != nil {
			yield(nil, fmt.Errorf("openai: marshal request: %w", err))
			return
		}

		slog.Debug("openai: request", "model", m.name, "endpoint", m.endpoint(), "stream", true, "body_len", len(body))

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint(), bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("openai: create request: %w", err))
			return
		}
		m.setHeaders(httpReq)

		start := time.Now()
		httpResp, err := m.client.Do(httpReq)
		if err != nil {
			slog.Error("openai: HTTP error", "model", m.name, "elapsed", time.Since(start), "err", err)
			yield(nil, fmt.Errorf("openai: HTTP %s: %w", m.name, err))
			return
		}
		defer httpResp.Body.Close()

		slog.Debug("openai: stream connected", "model", m.name, "status", httpResp.StatusCode, "elapsed", time.Since(start))

		if httpResp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(httpResp.Body)
			var apiErr ChatError
			_ = json.Unmarshal(respBody, &apiErr)
			slog.Error("openai: API error", "model", m.name, "status", httpResp.StatusCode, "type", apiErr.Error.Type, "message", apiErr.Error.Message)
			yield(nil, fmt.Errorf("openai: HTTP %d: %s: %s",
				httpResp.StatusCode, apiErr.Error.Type, apiErr.Error.Message))
			return
		}

		m.processSSE(httpResp.Body, yield)
	}
}

// processSSE reads the SSE stream and yields partial/final LLMResponses.
func (m *Model) processSSE(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Accumulate tool call arguments across deltas.
	type pendingTC struct {
		id   string
		name string
		args strings.Builder
	}
	var pendingCalls []pendingTC
	var finalUsage *ChatUsage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Text delta.
		if len(delta.Content) > 0 {
			var text string
			_ = json.Unmarshal(delta.Content, &text)
			if text != "" {
				resp := &model.LLMResponse{
					Content: newModelContent(&genai.Part{Text: text}),
					Partial: true,
				}
				if !yield(resp, nil) {
					return
				}
			}
		}

		// Tool call deltas.
		for _, tc := range delta.ToolCalls {
			// Extend pending list if needed.
			for len(pendingCalls) <= tc.Index {
				pendingCalls = append(pendingCalls, pendingTC{})
			}
			p := &pendingCalls[tc.Index]
			if tc.ID != "" {
				p.id = tc.ID
			}
			if tc.Function.Name != "" {
				p.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				p.args.WriteString(tc.Function.Arguments)
			}
		}

		// Finish reason signals end.
		if choice.FinishReason != "" {
			// Emit accumulated tool calls.
			if len(pendingCalls) > 0 {
				content := &genai.Content{Role: "model"}
				for _, p := range pendingCalls {
					var args map[string]any
					if s := p.args.String(); s != "" {
						_ = json.Unmarshal([]byte(s), &args)
					}
					content.Parts = append(content.Parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							ID:   p.id,
							Name: p.name,
							Args: args,
						},
					})
				}
				if !yield(&model.LLMResponse{Content: content}, nil) {
					return
				}
			}

			final := &model.LLMResponse{
				TurnComplete: true,
				FinishReason: MapFinishReason(choice.FinishReason),
			}
			if finalUsage != nil {
				final.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
					PromptTokenCount:     int32(finalUsage.PromptTokens),
					CandidatesTokenCount: int32(finalUsage.CompletionTokens),
					TotalTokenCount:      int32(finalUsage.TotalTokens),
				}
			}
			yield(final, nil)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		yield(nil, fmt.Errorf("openai: read stream: %w", err))
		return
	}

	// Stream ended without [DONE] or finish_reason — premature termination.
	yield(nil, fmt.Errorf("openai: stream ended prematurely without [DONE]"))
}
