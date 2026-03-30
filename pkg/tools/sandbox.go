package tools

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SandboxLevel controls the strictness of the sandbox.
type SandboxLevel string

const (
	SandboxNone      SandboxLevel = "none"
	SandboxStandard  SandboxLevel = "standard"  // Landlock kernel sandbox
	SandboxStrict    SandboxLevel = "strict"     // Landlock strict mode
	SandboxGVisor    SandboxLevel = "gvisor"     // gVisor runsc do (lightweight, no Docker)
	SandboxContainer SandboxLevel = "container"  // Docker + gVisor container (full isolation)
)

// SandboxOpts configures the sandbox wrapper for exec commands.
type SandboxOpts struct {
	Level        SandboxLevel // "none", "standard", "strict", or "container"
	HelperBinary string       // explicit path to abot-sandbox (for Landlock modes)

	// Container mode options (used when Level == SandboxContainer).
	ContainerImage   string // Docker image for sandbox (default: "abot/sandbox:latest")
	ContainerRuntime string // OCI runtime, e.g. "runsc" for gVisor (empty = Docker default)
	ContainerBinary  string // path to docker/nerdctl/podman binary (default: "docker")
	ContainerMemMB   int    // per-container memory limit in MB (default 512)
	ContainerCPUs    string // CPU quota, e.g. "0.5" (default "1")
	ContainerPids    int    // max PIDs inside container (default 256)
	ContainerNetwork string // "none", "host", or Docker network name (default "none")
	ContainerTmpMB         int    // tmpfs /tmp size in MB (default 100)
	ContainerDiskMB        int    // workspace overlay tmpfs in MB (0 = direct bind-mount)
	ContainerWorkspaceRoot string // host-side workspace root for DooD mode

	// gVisor standalone mode options (used when Level == SandboxGVisor).
	GVisorBinary  string // explicit path to runsc (auto-detected if empty)
	GVisorNetwork bool   // true = allow host network (default: isolated)
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
	if opts == nil || opts.Level == SandboxNone || opts.Level == "" {
		return "sh", []string{"-c", shellCmd}, false
	}

	if opts.Level == SandboxContainer {
		return wrapWithContainer(shellCmd, wsDir, opts)
	}

	if opts.Level == SandboxGVisor {
		return wrapWithGVisor(shellCmd, wsDir, opts)
	}

	// Landlock modes require Linux.
	if runtime.GOOS != "linux" {
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
