package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const (
	defaultExecTimeout = 60 * time.Second
	maxOutputLen       = 10000
)

type execArgs struct {
	Command string `json:"command" jsonschema:"Shell command to execute"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 60)"`
}

type execResult struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

func newExec(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "exec",
		Description: "Execute a shell command in the workspace directory with timeout and security filtering.",
	}, func(ctx tool.Context, args execArgs) (execResult, error) {
		// Security checks
		if err := ValidateCommand(args.Command, deps.DenyPatterns); err != nil {
			return execResult{Error: err.Error()}, nil
		}
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		if err := os.MkdirAll(wsDir, 0o755); err != nil {
			return execResult{Error: fmt.Sprintf("create workspace dir: %v", err)}, nil
		}
		if err := ValidateWorkspaceCommand(args.Command, wsDir); err != nil {
			return execResult{Error: err.Error()}, nil
		}

		timeout := defaultExecTimeout
		if args.Timeout > 0 {
			timeout = time.Duration(args.Timeout) * time.Second
		}
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Wrap command with ulimit resource constraints when configured.
		shellCmd := args.Command
		if deps.ExecLimits != nil {
			shellCmd = wrapWithLimits(shellCmd, deps.ExecLimits)
		}

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(execCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", shellCmd)
		} else {
			cmd = exec.CommandContext(execCtx, "sh", "-c", shellCmd)
		}
		cmd.Dir = wsDir
		cmd.WaitDelay = 3 * time.Second // kill lingering child I/O after timeout
		setProcGroup(cmd)               // Unix: Setpgid for process group kill

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return execResult{Error: fmt.Sprintf("exec failed: %v", err)}, nil
			}
		}

		return execResult{
			Stdout:   Truncate(stdout.String(), maxOutputLen),
			Stderr:   Truncate(stderr.String(), maxOutputLen),
			ExitCode: exitCode,
		}, nil
	})
	return t
}

// wrapWithLimits prepends ulimit constraints to a shell command string.
// Only effective on Unix; on Windows the original command is returned unchanged.
func wrapWithLimits(cmd string, limits *ExecLimits) string {
	if runtime.GOOS == "windows" || limits == nil {
		return cmd
	}
	var parts []string
	if limits.MemoryMB > 0 {
		parts = append(parts, fmt.Sprintf("ulimit -v %d", limits.MemoryMB*1024))
	}
	if limits.CPUSeconds > 0 {
		parts = append(parts, fmt.Sprintf("ulimit -t %d", limits.CPUSeconds))
	}
	if limits.FileSizeMB > 0 {
		// ulimit -f uses 512-byte blocks
		parts = append(parts, fmt.Sprintf("ulimit -f %d", limits.FileSizeMB*2048))
	}
	if limits.NProc > 0 {
		parts = append(parts, fmt.Sprintf("ulimit -u %d", limits.NProc))
	}
	if len(parts) == 0 {
		return cmd
	}
	return strings.Join(parts, "; ") + "; " + cmd
}

// Truncate returns s unchanged if its length is within max bytes.
// Otherwise it truncates at a valid UTF-8 boundary and appends an ellipsis marker.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// Find a valid UTF-8 boundary to avoid splitting a multi-byte character.
	Truncated := s[:max]
	for i := len(Truncated) - 1; i >= len(Truncated)-4 && i >= 0; i-- {
		if utf8.RuneStart(Truncated[i]) {
			r, size := utf8.DecodeRuneInString(Truncated[i:])
			if r == utf8.RuneError || i+size > len(Truncated) {
				Truncated = Truncated[:i]
			}
			break
		}
	}
	return Truncated + "\n... (Truncated)"
}
