package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"abot/pkg/mcp"
)

func TestJSONRPCRoundTrip(t *testing.T) {
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{"cursor": ""},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var decoded mcp.JSONRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: %q", decoded.JSONRPC)
	}
	if decoded.ID != 1 {
		t.Errorf("id: %d", decoded.ID)
	}
	if decoded.Method != "tools/list" {
		t.Errorf("method: %q", decoded.Method)
	}

	// Test response round-trip.
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"tools":[]}`),
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var decodedResp mcp.JSONRPCResponse
	if err := json.Unmarshal(respData, &decodedResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if decodedResp.ID != 1 {
		t.Errorf("response id: %d", decodedResp.ID)
	}
	if decodedResp.Error != nil {
		t.Errorf("unexpected error: %+v", decodedResp.Error)
	}

	// Test error response.
	errResp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      2,
		Error:   &mcp.JSONRPCError{Code: -32600, Message: "Invalid Request"},
	}
	errData, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}

	var decodedErr mcp.JSONRPCResponse
	if err := json.Unmarshal(errData, &decodedErr); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if decodedErr.Error == nil {
		t.Fatal("expected error in response")
	}
	if decodedErr.Error.Code != -32600 {
		t.Errorf("error code: %d", decodedErr.Error.Code)
	}
	if decodedErr.Error.Message != "Invalid Request" {
		t.Errorf("error message: %q", decodedErr.Error.Message)
	}
}

func TestConnectAll_NoServers(t *testing.T) {
	clients, tools, err := mcp.ConnectAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}

	// Also test with empty map.
	clients, tools, err = mcp.ConnectAll(context.Background(), map[string]mcp.ServerConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	c := mcp.NewClient("test", mcp.ServerConfig{Command: "echo"})
	if c.Config().ToolTimeout != 30 {
		t.Errorf("expected default timeout 30, got %d", c.Config().ToolTimeout)
	}
}

func TestNewClient_CustomTimeout(t *testing.T) {
	c := mcp.NewClient("test", mcp.ServerConfig{Command: "echo", ToolTimeout: 60})
	if c.Config().ToolTimeout != 60 {
		t.Errorf("expected timeout 60, got %d", c.Config().ToolTimeout)
	}
}

// setupStdioClient creates a Client wired to io.Pipe for testing the stdio protocol
// without spawning a real process. Returns the client plus the server-side reader/writer.
func setupStdioClient(t *testing.T) (c *mcp.Client, serverReader *bufio.Reader, serverWriter io.Writer) {
	t.Helper()
	// client writes to clientW -> serverReader reads
	clientR, serverW := io.Pipe()
	// server writes to serverWriter -> clientReader reads
	serverR, clientW := io.Pipe()

	c = mcp.NewClient("test-stdio", mcp.ServerConfig{Command: "fake", ToolTimeout: 5})
	c.SetStdio(clientW, bufio.NewReader(clientR))

	return c, bufio.NewReader(serverR), serverW
}

// respondJSON reads one JSON-RPC request from the server side and writes a response.
func respondJSON(t *testing.T, reader *bufio.Reader, writer io.Writer, result any, rpcErr *mcp.JSONRPCError) {
	t.Helper()
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		t.Fatalf("server unmarshal: %v", err)
	}

	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}
	if result != nil {
		raw, _ := json.Marshal(result)
		resp.Result = raw
	}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("server write: %v", err)
	}
}

func TestToolsListResult_Unmarshal(t *testing.T) {
	raw := `{"tools":[{"name":"search","description":"Search the web","inputSchema":{"type":"object","properties":{"q":{"type":"string"}}}}]}`
	var result mcp.TestToolsListResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "search" {
		t.Errorf("tool name: %q", result.Tools[0].Name)
	}
	if result.Tools[0].Description != "Search the web" {
		t.Errorf("tool description: %q", result.Tools[0].Description)
	}
}

// --- Stdio protocol simulation tests ---

func TestStdio_InitializeAndListTools(t *testing.T) {
	c, srvReader, srvWriter := setupStdioClient(t)

	done := make(chan error, 1)
	go func() {
		// 1. Respond to initialize
		respondJSON(t, srvReader, srvWriter, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"serverInfo":      map[string]any{"name": "test-server", "version": "1.0"},
		}, nil)

		// 2. Read the notifications/initialized notification (no response needed)
		_, _ = srvReader.ReadBytes('\n')

		// 3. Respond to tools/list
		respondJSON(t, srvReader, srvWriter, mcp.TestToolsListResult{
			Tools: []mcp.TestMCPToolDef{
				{Name: "echo", Description: "Echo input", InputSchema: map[string]any{"type": "object"}},
				{Name: "add", Description: "Add numbers", InputSchema: map[string]any{"type": "object"}},
			},
		}, nil)
		done <- nil
	}()

	ctx := context.Background()
	if err := c.TestInitialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := c.TestListTools(ctx); err != nil {
		t.Fatalf("listTools: %v", err)
	}

	<-done

	if len(c.Tools()) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(c.Tools()))
	}
	if c.Tools()[0].Name != "echo" {
		t.Errorf("tool[0] name: %q", c.Tools()[0].Name)
	}
	if c.Tools()[1].Name != "add" {
		t.Errorf("tool[1] name: %q", c.Tools()[1].Name)
	}
}

func TestStdio_CallTool(t *testing.T) {
	c, srvReader, srvWriter := setupStdioClient(t)

	go func() {
		respondJSON(t, srvReader, srvWriter, mcp.TestToolCallResult{
			Content: []mcp.TestContentBlock{
				{Type: "text", Text: "hello from tool"},
			},
		}, nil)
	}()

	result, err := c.CallTool(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "hello from tool" {
		t.Errorf("got %q, want %q", result, "hello from tool")
	}
}

func TestStdio_SkipsNotifications(t *testing.T) {
	c, srvReader, srvWriter := setupStdioClient(t)

	go func() {
		// Read the request from client
		line, _ := srvReader.ReadBytes('\n')
		var req mcp.JSONRPCRequest
		json.Unmarshal(line, &req)

		// Send 3 notifications (ID=0) before the real response
		for i := 0; i < 3; i++ {
			notif, _ := json.Marshal(mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      0,
				Result:  json.RawMessage(`{"type":"progress"}`),
			})
			srvWriter.Write(append(notif, '\n'))
		}

		// Now send the real response
		result, _ := json.Marshal(mcp.TestToolCallResult{
			Content: []mcp.TestContentBlock{{Type: "text", Text: "after notifications"}},
		})
		resp, _ := json.Marshal(mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		})
		srvWriter.Write(append(resp, '\n'))
	}()

	got, err := c.CallTool(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if got != "after notifications" {
		t.Errorf("got %q, want %q", got, "after notifications")
	}
}

func TestStdio_CallToolError(t *testing.T) {
	c, srvReader, srvWriter := setupStdioClient(t)

	go func() {
		respondJSON(t, srvReader, srvWriter, nil,
			&mcp.JSONRPCError{Code: -32000, Message: "tool not found"})
	}()

	_, err := c.CallTool(context.Background(), "missing", nil)
	if err == nil {
		t.Fatal("expected error from CallTool")
	}
	if !strings.Contains(err.Error(), "tool not found") {
		t.Errorf("error should mention 'tool not found': %v", err)
	}
}

// --- HTTP mode tests ---

func TestHTTP_SendRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req mcp.JSONRPCRequest
		json.Unmarshal(body, &req)

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "http-test"},
			}
		case "tools/list":
			result = mcp.TestToolsListResult{
				Tools: []mcp.TestMCPToolDef{
					{Name: "ping", Description: "Ping"},
				},
			}
		case "tools/call":
			result = mcp.TestToolCallResult{
				Content: []mcp.TestContentBlock{
					{Type: "text", Text: "pong"},
				},
			}
		}

		raw, _ := json.Marshal(result)
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  raw,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := mcp.NewClient("http-test", mcp.ServerConfig{
		URL:         srv.URL,
		ToolTimeout: 5,
	})
	c.SetHTTP(srv.URL, srv.Client(), map[string]string{})

	ctx := context.Background()

	// Initialize
	if err := c.TestInitialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// List tools
	if err := c.TestListTools(ctx); err != nil {
		t.Fatalf("listTools: %v", err)
	}
	if len(c.Tools()) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(c.Tools()))
	}
	if c.Tools()[0].Name != "ping" {
		t.Errorf("tool name: %q", c.Tools()[0].Name)
	}

	// Call tool
	result, err := c.CallTool(ctx, "ping", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "pong" {
		t.Errorf("got %q, want %q", result, "pong")
	}
}
