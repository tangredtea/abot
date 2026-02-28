package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// defaultToolTimeout is the default per-tool call timeout in seconds.
	defaultToolTimeout = 30
	// processWaitDelay is the grace period for child process I/O after context cancel.
	processWaitDelay = 3 * time.Second
	// maxSkipLines is the maximum number of non-matching lines to skip
	// before giving up when reading a stdio JSON-RPC response.
	maxSkipLines = 50
)

// Client manages connection to a single MCP server (stdio or HTTP).
type Client struct {
	name   string
	config ServerConfig

	// stdio mode
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	// http mode
	httpURL     string
	httpClient  *http.Client
	httpHeaders map[string]string

	// shared
	mu     sync.Mutex
	nextID int
	tools  []mcpToolDef
}

// NewClient creates a new MCP client for the given server config.
func NewClient(name string, cfg ServerConfig) *Client {
	if cfg.ToolTimeout <= 0 {
		cfg.ToolTimeout = defaultToolTimeout
	}
	return &Client{
		name:   name,
		config: cfg,
	}
}

// Connect starts the MCP server process (stdio) or validates the HTTP endpoint,
// then performs initialize + tools/list.
func (c *Client) Connect(ctx context.Context) error {
	if c.config.URL != "" {
		return c.connectHTTP(ctx)
	}
	if c.config.Command != "" {
		return c.connectStdio(ctx)
	}
	return fmt.Errorf("mcp server %q: no command or url configured", c.name)
}

func (c *Client) connectStdio(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	for k, v := range c.config.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// Ensure child process tree is killed on timeout/cancel.
	cmd.WaitDelay = processWaitDelay
	setMCPProcGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp %q: stdin pipe: %w", c.name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp %q: stdout pipe: %w", c.name, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp %q: start process: %w", c.name, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)

	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		return err
	}
	return c.listTools(ctx)
}

func (c *Client) connectHTTP(ctx context.Context) error {
	c.httpURL = strings.TrimRight(c.config.URL, "/")
	c.httpHeaders = c.config.Headers
	c.httpClient = &http.Client{
		Timeout: time.Duration(c.config.ToolTimeout*2) * time.Second,
	}

	if err := c.initialize(ctx); err != nil {
		return err
	}
	return c.listTools(ctx)
}

// Close shuts down the MCP server process if running.
func (c *Client) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// Tools returns the discovered MCP tool definitions.
func (c *Client) Tools() []mcpToolDef {
	return c.tools
}

// CallTool invokes a tool on the MCP server with timeout.
func (c *Client) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	timeout := time.Duration(c.config.ToolTimeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := c.sendRequest(ctx, "tools/call", toolCallParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp %q: call tool %q: %w", c.name, toolName, err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("mcp %q: tool %q error %d: %s",
			c.name, toolName, resp.Error.Code, resp.Error.Message)
	}

	var result toolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("mcp %q: unmarshal tool result: %w", c.name, err)
	}

	var parts []string
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return "(no output)", nil
	}
	return strings.Join(parts, "\n"), nil
}

// --- internal methods ---

func (c *Client) initialize(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]any{},
		ClientInfo: clientInfo{
			Name:    "abot",
			Version: "0.1.0",
		},
	})
	if err != nil {
		return fmt.Errorf("mcp %q: initialize: %w", c.name, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("mcp %q: initialize error %d: %s",
			c.name, resp.Error.Code, resp.Error.Message)
	}

	// Send initialized notification (no response expected).
	// For stdio, we write it; for HTTP, we POST it.
	notif := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)

	if c.httpURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.httpURL, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			for k, v := range c.httpHeaders {
				req.Header.Set(k, v)
			}
			resp, err := c.httpClient.Do(req)
			if err != nil {
				slog.Warn("mcp: initialized notification failed", "server", c.name, "err", err)
			} else {
				resp.Body.Close()
			}
		}
	} else if c.stdin != nil {
		data = append(data, '\n')
		if _, err := c.stdin.Write(data); err != nil {
			slog.Warn("mcp: initialized notification failed", "server", c.name, "err", err)
		}
	}

	return nil
}

func (c *Client) listTools(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("mcp %q: list tools: %w", c.name, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("mcp %q: list tools error %d: %s",
			c.name, resp.Error.Code, resp.Error.Message)
	}

	var result toolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("mcp %q: unmarshal tools list: %w", c.name, err)
	}

	c.tools = result.Tools
	slog.Info("mcp: tools discovered", "server", c.name, "count", len(c.tools))
	return nil
}

func (c *Client) sendRequest(ctx context.Context, method string, params any) (*JSONRPCResponse, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if c.httpURL != "" {
		return c.sendHTTP(ctx, &req)
	}
	return c.sendStdio(ctx, &req)
}

func (c *Client) sendStdio(_ context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	// Read lines until we get a response matching our request ID.
	// MCP servers may send notifications (ID=0) between request and response.
	for i := 0; i < maxSkipLines; i++ {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("read from stdout: %w", err)
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Debug("mcp: skipping unparseable line", "server", c.name)
			continue
		}

		// ID=0 means notification (our IDs start at 1). Skip it.
		if resp.ID == 0 {
			slog.Debug("mcp: skipping notification", "server", c.name)
			continue
		}

		if resp.ID != req.ID {
			slog.Warn("mcp: unexpected response ID",
				"server", c.name, "want", req.ID, "got", resp.ID)
			continue
		}

		return &resp, nil
	}
	return nil, fmt.Errorf("mcp %q: no matching response after %d lines", c.name, maxSkipLines)
}

func (c *Client) sendHTTP(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.httpURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.httpHeaders {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read http response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", httpResp.StatusCode, string(body))
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}
