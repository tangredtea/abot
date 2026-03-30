package tools

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultContainerImage = "abot/sandbox:latest"
	defaultContainerMem   = 512
	defaultContainerCPUs  = "1"
	defaultContainerPids  = 256
	defaultContainerTmpMB = 100
)

// wrapWithContainer returns a Docker/Podman command that runs shellCmd
// inside an isolated container, optionally using the gVisor (runsc) runtime.
//
// Security layers applied:
//   - OCI runtime isolation (runsc/runc) — kernel-level syscall interception
//   - cgroup resource limits (memory, CPU, PIDs)
//   - read-only root filesystem + size-limited tmpfs for /tmp and $HOME
//   - no-new-privileges security option
//   - network isolation (default: none)
//   - workspace bind-mount as the only writable host path
//   - non-root user (1000:1000)
func wrapWithContainer(shellCmd, wsDir string, opts *SandboxOpts) (bin string, args []string, sandboxed bool) {
	dockerBin := resolveContainerBinary(opts.ContainerBinary)
	if dockerBin == "" {
		slog.Warn("sandbox: container binary not found, falling back to unsandboxed execution")
		return "sh", []string{"-c", shellCmd}, false
	}

	image := opts.ContainerImage
	if image == "" {
		image = defaultContainerImage
	}

	args = []string{"run", "--rm"}

	// OCI runtime (gVisor).
	if opts.ContainerRuntime != "" {
		args = append(args, "--runtime="+opts.ContainerRuntime)
	}

	// Resource limits.
	mem := opts.ContainerMemMB
	if mem <= 0 {
		mem = defaultContainerMem
	}
	args = append(args, fmt.Sprintf("--memory=%dm", mem))

	cpus := opts.ContainerCPUs
	if cpus == "" {
		cpus = defaultContainerCPUs
	}
	args = append(args, "--cpus="+cpus)

	pids := opts.ContainerPids
	if pids <= 0 {
		pids = defaultContainerPids
	}
	args = append(args, fmt.Sprintf("--pids-limit=%d", pids))

	// Network isolation.
	network := opts.ContainerNetwork
	if network == "" {
		network = "none"
	}
	args = append(args, "--network="+network)

	// Read-only root FS + tmpfs for scratch areas.
	args = append(args, "--read-only")
	args = append(args, "--security-opt=no-new-privileges")

	tmpMB := opts.ContainerTmpMB
	if tmpMB <= 0 {
		tmpMB = defaultContainerTmpMB
	}
	args = append(args, fmt.Sprintf("--tmpfs=/tmp:size=%dm,exec", tmpMB))

	// Writable $HOME (npm/pip cache, dotfiles) — isolated per container run.
	args = append(args, fmt.Sprintf("--tmpfs=/home/sandbox:size=%dm", tmpMB))

	// Environment: redirect tool caches into /tmp so read-only FS won't block them.
	args = append(args,
		"-e", "HOME=/home/sandbox",
		"-e", "NPM_CONFIG_CACHE=/tmp/.npm",
		"-e", "PIP_CACHE_DIR=/tmp/.pip",
		"-e", "YARN_CACHE_FOLDER=/tmp/.yarn",
	)

	// Workspace mount — the only host-writable path.
	// In DooD (Docker-outside-of-Docker) mode, the abot process runs inside
	// a container but spawns sandbox containers via the host Docker socket.
	// The host daemon cannot see container-internal paths, so we translate
	// the workspace path using ContainerWorkspaceRoot (the host-side mount).
	if wsDir != "" {
		absWs, err := filepath.Abs(wsDir)
		if err == nil {
			wsDir = absWs
		}
		hostWs := resolveHostWorkspace(wsDir, opts.ContainerWorkspaceRoot)
		args = append(args, "-v", hostWs+":/workspace")
	}
	args = append(args, "-w", "/workspace")

	// Non-root user.
	args = append(args, "--user", "1000:1000")

	// Image + command.
	args = append(args, image, "sh", "-c", shellCmd)

	return dockerBin, args, true
}

// resolveHostWorkspace translates a container-internal workspace path to the
// host-side path when running in DooD mode. If hostRoot is empty (bare-metal
// deployment), the original path is returned unchanged.
//
// Example: wsDir="/app/workspace/t1/u1", hostRoot="/data/abot/workspace"
// The function finds "workspace" as the common base and returns
// "/data/abot/workspace/t1/u1".
func resolveHostWorkspace(wsDir, hostRoot string) string {
	if hostRoot == "" {
		return wsDir
	}
	// hostRoot maps to the WorkspaceDir base (e.g. "workspace" or "/app/workspace").
	// We need the relative path from the workspace base to the tenant/user dir.
	// Strategy: find "workspace" directory name in wsDir and take everything after.
	const marker = "workspace"
	idx := strings.Index(wsDir, marker)
	if idx < 0 {
		return wsDir
	}
	suffix := wsDir[idx+len(marker):]
	return filepath.Join(hostRoot, suffix)
}

// resolveContainerBinary locates docker/nerdctl/podman on the host.
func resolveContainerBinary(explicit string) string {
	if explicit != "" {
		if p, err := exec.LookPath(explicit); err == nil {
			return p
		}
		return ""
	}
	for _, name := range []string{"docker", "nerdctl", "podman"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}
