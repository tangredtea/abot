// Package debug provides debugging and tracing utilities for abot.
package debug

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Tracer provides debugging output for agent operations.
type Tracer struct {
	enabled bool
	writer  io.Writer
}

// NewTracer creates a new debug tracer.
func NewTracer(enabled bool) *Tracer {
	return &Tracer{
		enabled: enabled,
		writer:  os.Stderr,
	}
}

// Enabled returns whether debug mode is enabled.
func (t *Tracer) Enabled() bool {
	return t.enabled
}

// TraceToolCall logs a tool invocation.
func (t *Tracer) TraceToolCall(name string, input, output any, duration time.Duration) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "\n[TOOL] %s (%.2fs)\n", name, duration.Seconds())
	fmt.Fprintf(t.writer, "  Input:  %+v\n", input)
	fmt.Fprintf(t.writer, "  Output: %+v\n", output)
}

// TraceLLMCall logs an LLM request.
func (t *Tracer) TraceLLMCall(model string, tokens int, latency time.Duration, cost float64) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "\n[LLM] %s\n", model)
	fmt.Fprintf(t.writer, "  Tokens:  %d\n", tokens)
	fmt.Fprintf(t.writer, "  Latency: %v\n", latency)
	if cost > 0 {
		fmt.Fprintf(t.writer, "  Cost:    $%.4f\n", cost)
	}
}

// TraceEvent logs a generic event.
func (t *Tracer) TraceEvent(event string, details map[string]any) {
	if !t.enabled {
		return
	}
	fmt.Fprintf(t.writer, "\n[EVENT] %s\n", event)
	for k, v := range details {
		fmt.Fprintf(t.writer, "  %s: %+v\n", k, v)
	}
}
