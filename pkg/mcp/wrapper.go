package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// ConnectAll connects to all configured MCP servers and returns ADK tools.
func ConnectAll(ctx context.Context, servers map[string]ServerConfig) ([]*Client, []tool.Tool, error) {
	if len(servers) == 0 {
		return nil, nil, nil
	}

	var clients []*Client
	var tools []tool.Tool

	for name, cfg := range servers {
		client := NewClient(name, cfg)
		if err := client.Connect(ctx); err != nil {
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, nil, fmt.Errorf("mcp server %q: %w", name, err)
		}
		clients = append(clients, client)

		for _, def := range client.Tools() {
			t := wrapTool(client, name, def)
			tools = append(tools, t)
			slog.Debug("mcp: registered tool", "name", t.Name(), "server", name)
		}
		slog.Info("mcp: server connected", "server", name, "tools", len(client.Tools()))
	}

	return clients, tools, nil
}

// mcpCallArgs is the generic argument type for wrapped MCP tools.
type mcpCallArgs struct {
	Arguments map[string]any `json:"arguments,omitempty" jsonschema:"Tool arguments as key-value pairs"`
}

// mcpCallResult is the result type for wrapped MCP tools.
type mcpCallResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// wrapTool creates an ADK tool.Tool from an MCP tool definition using functiontool.New.
func wrapTool(client *Client, serverName string, def mcpToolDef) tool.Tool {
	toolName := fmt.Sprintf("mcp_%s_%s", serverName, def.Name)
	desc := def.Description
	if desc == "" {
		desc = def.Name
	}
	originalName := def.Name

	t, err := functiontool.New(functiontool.Config{
		Name:        toolName,
		Description: desc,
	}, func(ctx tool.Context, args json.RawMessage) (mcpCallResult, error) {
		var argsMap map[string]any
		if len(args) > 0 {
			if err := json.Unmarshal(args, &argsMap); err != nil {
				return mcpCallResult{Error: fmt.Sprintf("invalid args: %v", err)}, nil
			}
		}

		result, err := client.CallTool(context.Background(), originalName, argsMap)
		if err != nil {
			return mcpCallResult{Error: err.Error()}, nil
		}
		return mcpCallResult{Output: result}, nil
	})
	if err != nil {
		slog.Error("mcp: failed to create tool wrapper", "name", toolName, "err", err)
		return nil
	}
	return t
}
