package tools

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SandboxLevel controls the strictness of the Landlock filesystem sandbox.
type SandboxLevel string

const (
	SandboxNone     SandboxLevel = "none"
	SandboxStandard SandboxLevel = "standard"
	SandboxStrict   SandboxLevel = "strict"
)

// SandboxOpts configures the Landlock sandbox wrapper for exec commands.
type SandboxOpts struct {
	Level        SandboxLevel // "none", "standard", or "strict"
	HelperBinary string       // explicit path to abot-sandbox (auto-detected if empty)
}

// SandboxBinaryPath locates the abot-sandbox helper binary.
// Search order: explicit path → same directory as current executable → PATH.
func SandboxBinaryPath(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}

	// Same directory as the running executable.
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), "abot-sandbox")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Search PATH.
	if p, err := exec.LookPath("abot-sandbox"); err == nil {
		return p
	}

	return ""
}

// WrapWithSandbox returns the binary, args, and whether sandboxing is active.
// On non-Linux, when opts is nil/none, or when the helper is not found,
// it gracefully falls back to plain "sh -c cmd".
func WrapWithSandbox(shellCmd, wsDir string, opts *SandboxOpts) (bin string, args []string, sandboxed bool) {
	if runtime.GOOS != "linux" || opts == nil || opts.Level == SandboxNone || opts.Level == "" {
		return "sh", []string{"-c", shellCmd}, false
	}

	helperPath := SandboxBinaryPath(opts.HelperBinary)
	if helperPath == "" {
		slog.Warn("sandbox: abot-sandbox helper not found, running without sandbox")
		return "sh", []string{"-c", shellCmd}, false
	}

	return helperPath, []string{
		"--workspace=" + wsDir,
		"--level=" + string(opts.Level),
		"--", "sh", "-c", shellCmd,
	}, true
}
