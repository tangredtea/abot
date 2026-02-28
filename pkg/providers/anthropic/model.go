package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"google.golang.org/adk/model"
)

const (
	defaultAPIBase    = "https://api.anthropic.com"
	messagesEndpoint  = "/v1/messages"
	defaultAPIVersion = "2023-06-01"
)

// Model implements model.LLM for the Anthropic Messages API.
type Model struct {
	name           string
	apiKey         string
	apiBase        string
	apiVersion     string
	client         *http.Client
	promptCaching  bool
}

// Config holds Anthropic model configuration.
type Config struct {
	Name           string // model name, e.g. "claude-sonnet-4-20250514"
	APIKey         string
	APIBase        string // defaults to https://api.anthropic.com
	APIVersion     string // defaults to "2023-06-01"
	Client         *http.Client
	PromptCaching  bool   // enable Anthropic prompt caching
}

// NewModel creates an Anthropic model.LLM implementation.
func NewModel(cfg Config) *Model {
	base := cfg.APIBase
	if base == "" {
		base = defaultAPIBase
	}
	base = strings.TrimRight(base, "/")

	ver := cfg.APIVersion
	if ver == "" {
		ver = defaultAPIVersion
	}

	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}

	return &Model{
		name:          cfg.Name,
		apiKey:        cfg.APIKey,
		apiBase:       base,
		apiVersion:    ver,
		client:        client,
		promptCaching: cfg.PromptCaching,
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

// generate performs a synchronous (non-streaming) Messages API call.
func (m *Model) generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	apiReq, err := ConvertRequest(req, m.name, false, m.promptCaching)
	if err != nil {
		return nil, fmt.Errorf("anthropic: convert request: %w", err)
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiBase+messagesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	m.setHeaders(httpReq)

	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP %s: %w", m.name, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr apiError
		_ = json.Unmarshal(respBody, &apiErr)
		return nil, fmt.Errorf("anthropic: HTTP %d: %s: %s",
			httpResp.StatusCode, apiErr.Error.Type, apiErr.Error.Message)
	}

	var resp MessagesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	return ConvertResponse(&resp), nil
}

func (m *Model) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", m.apiVersion)
}

// generateStream performs a streaming Messages API call, yielding partial LLMResponses.
func (m *Model) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		apiReq, err := ConvertRequest(req, m.name, true, m.promptCaching)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: convert request: %w", err))
			return
		}

		body, err := json.Marshal(apiReq)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: marshal request: %w", err))
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiBase+messagesEndpoint, bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: create request: %w", err))
			return
		}
		m.setHeaders(httpReq)

		httpResp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("anthropic: HTTP %s: %w", m.name, err))
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(httpResp.Body)
			var apiErr apiError
			_ = json.Unmarshal(respBody, &apiErr)
			yield(nil, fmt.Errorf("anthropic: HTTP %d: %s: %s",
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

	// Track current tool_use block for JSON accumulation.
	var (
		currentBlockID   string
		currentBlockName string
		currentBlockType string
		accumulatedJSON  strings.Builder
		finalUsage       *APIUsage
		finalStopReason  string
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var evt streamEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "content_block_start":
			var block ContentBlock
			if err := json.Unmarshal(evt.ContentBlock, &block); err == nil {
				currentBlockType = block.Type
				currentBlockID = block.ID
				currentBlockName = block.Name
				accumulatedJSON.Reset()
			}

		case "content_block_delta":
			var delta streamDelta
			if err := json.Unmarshal(evt.Delta, &delta); err != nil {
				continue
			}
			if delta.Type == "text_delta" {
				resp := convertStreamDelta(&delta)
				if !yield(resp, nil) {
					return
				}
			} else if delta.Type == "input_json_delta" {
				accumulatedJSON.WriteString(delta.PartialJSON)
			}

		case "content_block_stop":
			if currentBlockType == "tool_use" && currentBlockID != "" {
				part := convertStreamToolUse(currentBlockID, currentBlockName, accumulatedJSON.String())
				resp := &model.LLMResponse{
					Content: newModelContent(part),
					Partial: false,
				}
				if !yield(resp, nil) {
					return
				}
			}
			currentBlockType = ""
			currentBlockID = ""
			currentBlockName = ""
			accumulatedJSON.Reset()

		case "message_delta":
			var delta streamDelta
			if err := json.Unmarshal(evt.Delta, &delta); err == nil {
				finalStopReason = delta.StopReason
			}
			if evt.Usage != nil {
				finalUsage = evt.Usage
			}

		case "message_stop":
			final := &model.LLMResponse{
				TurnComplete: true,
				FinishReason: MapStopReason(finalStopReason),
			}
			if finalUsage != nil {
				final.UsageMetadata = convertUsage(finalUsage)
			}
			yield(final, nil)
			return

		case "error":
			var errDetail struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &errDetail); err == nil {
				yield(nil, fmt.Errorf("anthropic: stream error: %s: %s",
					errDetail.Error.Type, errDetail.Error.Message))
			} else {
				yield(nil, fmt.Errorf("anthropic: stream error: %s", data))
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		yield(nil, fmt.Errorf("anthropic: read stream: %w", err))
	}
}
