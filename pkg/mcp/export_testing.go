//go:build testing

package mcp

import (
	"bufio"
	"context"
	"io"
	"net/http"
)

// Exported type aliases for external testing.

type TestToolsListResult = toolsListResult
type TestMCPToolDef = mcpToolDef
type TestToolCallResult = toolCallResult
type TestContentBlock = contentBlock

// SetStdio exposes stdin/stdout for testing the stdio protocol.
func (c *Client) SetStdio(w io.WriteCloser, r *bufio.Reader) {
	c.stdin = w
	c.stdout = r
}

// SetHTTP exposes HTTP fields for testing.
func (c *Client) SetHTTP(url string, client *http.Client, headers map[string]string) {
	c.httpURL = url
	c.httpClient = client
	c.httpHeaders = headers
}

// TestInitialize exposes the unexported initialize method.
func (c *Client) TestInitialize(ctx context.Context) error {
	return c.initialize(ctx)
}

// TestListTools exposes the unexported listTools method.
func (c *Client) TestListTools(ctx context.Context) error {
	return c.listTools(ctx)
}

// Config exposes the config field.
func (c *Client) Config() ServerConfig {
	return c.config
}
