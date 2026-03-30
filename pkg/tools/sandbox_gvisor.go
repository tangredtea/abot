package tools

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
)

// wrapWithGVisor uses "runsc do" to sandbox a command directly via gVisor's
// userspace kernel — no Docker daemon, no container image, no OCI overhead.
//
// This is the lightest gVisor mode:
//   - runsc intercepts all syscalls in userspace (~50-100ms startup)
//   - filesystem restricted to workspace via --root-mount / mount flags
//   - network isolated by default (runsc do uses its own network namespace)
//   - no Docker daemon required, no image pull, no container lifecycle
//
// Compared to container mode (docker run --runtime=runsc):
//   - ~5x faster startup (50ms vs 300ms)
//   - no Docker dependency
//   - but no cgroup resource limits (memory/CPU/PIDs) — use ulimit as fallback
//   - no custom image (uses host binaries visible under the mount)
func wrapWithGVisor(shellCmd, wsDir string, opts *SandboxOpts) (bin string, args []string, sandboxed bool) {
	if runtime.GOOS != "linux" {
		slog.Warn("sandbox: gvisor (runsc) only supported on Linux, falling back to unsandboxed")
		return "sh", []string{"-c", shellCmd}, false
	}

	runscBin := resolveRunsc(opts.GVisorBinary)
	if runscBin == "" {
		slog.Warn("sandbox: runsc binary not found, falling back to unsandboxed execution")
		return "sh", []string{"-c", shellCmd}, false
	}

	args = []string{"do"}

	// Network isolation (default: enabled in runsc do).
	if opts.GVisorNetwork {
		args = append(args, "--network=host")
	}

	// Mount workspace as writable root for the sandbox.
	if wsDir != "" {
		absWs, err := filepath.Abs(wsDir)
		if err == nil {
			wsDir = absWs
		}
		args = append(args, "--root="+wsDir)
	}

	args = append(args, "--", "sh", "-c", shellCmd)

	return runscBin, args, true
}

// resolveRunsc locates the runsc binary.
func resolveRunsc(explicit string) string {
	if explicit != "" {
		if p, err := exec.LookPath(explicit); err == nil {
			return p
		}
		return ""
	}
	if p, err := exec.LookPath("runsc"); err == nil {
		return p
	}
	return ""
}
