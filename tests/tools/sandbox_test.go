package tools_test

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"abot/pkg/tools"
)

func TestWrapWithSandbox_NilOpts(t *testing.T) {
	bin, args, sandboxed := tools.WrapWithSandbox("echo hello", "/tmp/ws", nil)
	if sandboxed {
		t.Error("expected sandboxed=false for nil opts")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hello" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestWrapWithSandbox_LevelNone(t *testing.T) {
	opts := &tools.SandboxOpts{Level: tools.SandboxNone}
	bin, args, sandboxed := tools.WrapWithSandbox("ls", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false for level=none")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
	if len(args) != 2 || args[0] != "-c" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestWrapWithSandbox_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("test only for non-Linux platforms")
	}
	opts := &tools.SandboxOpts{Level: tools.SandboxStandard}
	bin, _, sandboxed := tools.WrapWithSandbox("ls", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false on non-Linux")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
}

func TestWrapWithSandbox_HelperNotFound(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandbox only supported on Linux")
	}
	opts := &tools.SandboxOpts{
		Level:        tools.SandboxStandard,
		HelperBinary: "/nonexistent/abot-sandbox",
	}
	// Ensure the binary is NOT on PATH either by checking.
	if tools.SandboxBinaryPath("/nonexistent/abot-sandbox") != "" {
		t.Skip("abot-sandbox found on PATH, cannot test missing binary fallback")
	}
	bin, args, sandboxed := tools.WrapWithSandbox("ls", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false when helper not found")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "ls" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestWrapWithSandbox_ArgsStructure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandbox only supported on Linux")
	}
	// Create a temporary fake binary.
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/abot-sandbox"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := &tools.SandboxOpts{
		Level:        tools.SandboxStandard,
		HelperBinary: fakeBin,
	}
	bin, args, sandboxed := tools.WrapWithSandbox("echo hello", "/workspace/test", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true with valid helper binary")
	}
	if bin != fakeBin {
		t.Errorf("expected bin=%s, got %s", fakeBin, bin)
	}
	// Args should be: --workspace=/workspace/test --level=standard -- sh -c "echo hello"
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[0] != "--workspace=/workspace/test" {
		t.Errorf("args[0]: got %q", args[0])
	}
	if args[1] != "--level=standard" {
		t.Errorf("args[1]: got %q", args[1])
	}
	if args[2] != "--" {
		t.Errorf("args[2]: got %q", args[2])
	}
	if args[3] != "sh" || args[4] != "-c" || args[5] != "echo hello" {
		t.Errorf("args[3:6]: got %v", args[3:6])
	}
}

func TestSandboxBinaryPath_ExplicitPath(t *testing.T) {
	// Non-existent explicit path should return empty.
	got := tools.SandboxBinaryPath("/nonexistent/path/abot-sandbox")
	if got != "" {
		t.Errorf("expected empty for nonexistent path, got %s", got)
	}
}

func TestSandboxBinaryPath_EmptyExplicit(t *testing.T) {
	// Empty explicit should fall through to other search methods.
	// We can't guarantee it exists, just verify no panic.
	_ = tools.SandboxBinaryPath("")
}

// ---------------------------------------------------------------------------
// New test cases
// ---------------------------------------------------------------------------

func TestWrapWithSandbox_EmptyWsDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		// On non-Linux the sandbox path is never taken, so we test the
		// non-sandboxed fallback which still works with an empty wsDir.
		bin, args, sandboxed := tools.WrapWithSandbox("ls", "", &tools.SandboxOpts{Level: tools.SandboxStandard})
		if sandboxed {
			t.Error("expected sandboxed=false on non-Linux")
		}
		if bin != "sh" {
			t.Errorf("expected bin=sh, got %s", bin)
		}
		if len(args) != 2 || args[0] != "-c" || args[1] != "ls" {
			t.Errorf("unexpected args: %v", args)
		}
		return
	}
	// On Linux, create a fake helper so we exercise the sandbox path.
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/abot-sandbox"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := &tools.SandboxOpts{Level: tools.SandboxStandard, HelperBinary: fakeBin}
	bin, args, sandboxed := tools.WrapWithSandbox("ls", "", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true on Linux with valid helper")
	}
	if bin != fakeBin {
		t.Errorf("expected bin=%s, got %s", fakeBin, bin)
	}
	// The first arg should be "--workspace=" (empty wsDir).
	if args[0] != "--workspace=" {
		t.Errorf("expected args[0]=%q, got %q", "--workspace=", args[0])
	}
}

func TestWrapWithSandbox_WsDirWithSpaces(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandbox only supported on Linux")
	}
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/abot-sandbox"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wsDir := "/tmp/my work space/project"
	opts := &tools.SandboxOpts{Level: tools.SandboxStandard, HelperBinary: fakeBin}
	bin, args, sandboxed := tools.WrapWithSandbox("echo hi", wsDir, opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true")
	}
	if bin != fakeBin {
		t.Errorf("expected bin=%s, got %s", fakeBin, bin)
	}
	expectedWsArg := "--workspace=" + wsDir
	if args[0] != expectedWsArg {
		t.Errorf("expected args[0]=%q, got %q", expectedWsArg, args[0])
	}
	// Verify the space is preserved as part of the single argument.
	if !strings.Contains(args[0], " ") {
		t.Error("expected workspace arg to contain spaces")
	}
}

func TestWrapWithSandbox_EmptyLevel(t *testing.T) {
	// An empty Level should behave the same as SandboxNone (fallback).
	opts := &tools.SandboxOpts{Level: ""}
	bin, args, sandboxed := tools.WrapWithSandbox("pwd", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false for empty Level")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "pwd" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestWrapWithSandbox_StrictLevel(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sandbox only supported on Linux")
	}
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/abot-sandbox"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := &tools.SandboxOpts{Level: tools.SandboxStrict, HelperBinary: fakeBin}
	bin, args, sandboxed := tools.WrapWithSandbox("cat /etc/passwd", "/workspace/strict-test", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true for strict level")
	}
	if bin != fakeBin {
		t.Errorf("expected bin=%s, got %s", fakeBin, bin)
	}
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[1] != "--level=strict" {
		t.Errorf("expected args[1]=%q, got %q", "--level=strict", args[1])
	}
	if args[0] != "--workspace=/workspace/strict-test" {
		t.Errorf("expected args[0]=%q, got %q", "--workspace=/workspace/strict-test", args[0])
	}
	if args[2] != "--" || args[3] != "sh" || args[4] != "-c" || args[5] != "cat /etc/passwd" {
		t.Errorf("unexpected trailing args: %v", args[2:])
	}
}

func TestSandboxBinaryPath_SameDirAsExecutable(t *testing.T) {
	// We cannot easily control where os.Executable() resolves, but we can
	// verify the function does not panic for any state. The return value is
	// either a valid path or "".
	result := tools.SandboxBinaryPath("")
	if result != "" {
		// If something was found, verify the file actually exists.
		if _, err := os.Stat(result); err != nil {
			t.Errorf("sandboxBinaryPath returned %q but stat failed: %v", result, err)
		}
	}
}

// ---------------------------------------------------------------------------
// gVisor standalone tests (runsc do — no Docker)
// ---------------------------------------------------------------------------

func TestWrapWithSandbox_GVisorMode_NonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("test only for non-Linux platforms")
	}
	opts := &tools.SandboxOpts{Level: tools.SandboxGVisor}
	bin, _, sandboxed := tools.WrapWithSandbox("echo hello", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false on non-Linux")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
}

func TestWrapWithSandbox_GVisorMode_NoRunsc(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("gvisor only on Linux")
	}
	opts := &tools.SandboxOpts{
		Level:        tools.SandboxGVisor,
		GVisorBinary: "/nonexistent/runsc",
	}
	bin, _, sandboxed := tools.WrapWithSandbox("ls", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false when runsc not found")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
}

func TestWrapWithSandbox_GVisorMode_Args(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("gvisor only on Linux")
	}
	tmpDir := t.TempDir()
	fakeRunsc := tmpDir + "/runsc"
	if err := os.WriteFile(fakeRunsc, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := &tools.SandboxOpts{
		Level:        tools.SandboxGVisor,
		GVisorBinary: fakeRunsc,
	}
	bin, args, sandboxed := tools.WrapWithSandbox("npm install", "/workspace/t1/u1", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true")
	}
	if bin != fakeRunsc {
		t.Errorf("expected bin=%s, got %s", fakeRunsc, bin)
	}

	joined := strings.Join(args, " ")

	for _, want := range []string{
		"do",
		"--root=/workspace/t1/u1",
		"-- sh -c npm install",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q, got: %s", want, joined)
		}
	}

	// Default: no --network=host
	if strings.Contains(joined, "--network=host") {
		t.Errorf("should not have --network=host by default, got: %s", joined)
	}
}

func TestWrapWithSandbox_GVisorMode_WithNetwork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("gvisor only on Linux")
	}
	tmpDir := t.TempDir()
	fakeRunsc := tmpDir + "/runsc"
	if err := os.WriteFile(fakeRunsc, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := &tools.SandboxOpts{
		Level:         tools.SandboxGVisor,
		GVisorBinary:  fakeRunsc,
		GVisorNetwork: true,
	}
	_, args, sandboxed := tools.WrapWithSandbox("curl example.com", "/tmp/ws", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true")
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--network=host") {
		t.Errorf("expected --network=host, got: %s", joined)
	}
}

// ---------------------------------------------------------------------------
// Container sandbox tests (Docker + gVisor)
// ---------------------------------------------------------------------------

func TestWrapWithSandbox_ContainerMode_NoDocker(t *testing.T) {
	opts := &tools.SandboxOpts{
		Level:           tools.SandboxContainer,
		ContainerBinary: "/nonexistent/docker-binary",
	}
	bin, args, sandboxed := tools.WrapWithSandbox("echo hello", "/tmp/ws", opts)
	if sandboxed {
		t.Error("expected sandboxed=false when container binary not found")
	}
	if bin != "sh" {
		t.Errorf("expected bin=sh, got %s", bin)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hello" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestWrapWithSandbox_ContainerMode_ArgsStructure(t *testing.T) {
	// Create a fake docker binary to make resolveContainerBinary succeed.
	tmpDir := t.TempDir()
	fakeDocker := tmpDir + "/docker"
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Prepend tmpDir to PATH so LookPath finds our fake binary.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	opts := &tools.SandboxOpts{
		Level:            tools.SandboxContainer,
		ContainerImage:   "myregistry/sandbox:v2",
		ContainerRuntime: "runsc",
		ContainerMemMB:   256,
		ContainerCPUs:    "0.5",
		ContainerPids:    128,
		ContainerNetwork: "abot-net",
		ContainerTmpMB:   50,
	}

	bin, args, sandboxed := tools.WrapWithSandbox("npm install", "/workspace/tenant1/user1", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true for container mode")
	}
	if bin != fakeDocker {
		t.Errorf("expected bin=%s, got %s", fakeDocker, bin)
	}

	joined := strings.Join(args, " ")

	// Verify key flags.
	for _, want := range []string{
		"run --rm",
		"--runtime=runsc",
		"--memory=256m",
		"--cpus=0.5",
		"--pids-limit=128",
		"--network=abot-net",
		"--read-only",
		"--security-opt=no-new-privileges",
		"--tmpfs=/tmp:size=50m,exec",
		"-w /workspace",
		"--user 1000:1000",
		"myregistry/sandbox:v2",
		"sh -c npm install",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q, got: %s", want, joined)
		}
	}
}

func TestWrapWithSandbox_ContainerMode_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	fakeDocker := tmpDir + "/docker"
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	opts := &tools.SandboxOpts{Level: tools.SandboxContainer}
	_, args, sandboxed := tools.WrapWithSandbox("ls", "/tmp/ws", opts)
	if !sandboxed {
		t.Fatal("expected sandboxed=true")
	}

	joined := strings.Join(args, " ")

	// Verify defaults.
	for _, want := range []string{
		"--memory=512m",    // default mem
		"--cpus=1",         // default cpus
		"--pids-limit=256", // default pids
		"--network=none",   // default network
		"--tmpfs=/tmp:size=100m,exec",
		"abot/sandbox:latest", // default image
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing default %q, got: %s", want, joined)
		}
	}

	// Should NOT contain --runtime when ContainerRuntime is empty.
	if strings.Contains(joined, "--runtime=") {
		t.Errorf("should not have --runtime when runtime is empty, got: %s", joined)
	}
}

func TestWrapWithSandbox_ContainerMode_EnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	fakeDocker := tmpDir + "/docker"
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	opts := &tools.SandboxOpts{Level: tools.SandboxContainer}
	_, args, _ := tools.WrapWithSandbox("node -v", "/tmp/ws", opts)
	joined := strings.Join(args, " ")

	for _, env := range []string{
		"HOME=/home/sandbox",
		"NPM_CONFIG_CACHE=/tmp/.npm",
		"PIP_CACHE_DIR=/tmp/.pip",
	} {
		if !strings.Contains(joined, env) {
			t.Errorf("args missing env %q, got: %s", env, joined)
		}
	}
}

func TestWrapWithSandbox_ContainerMode_WorkspaceMount(t *testing.T) {
	tmpDir := t.TempDir()
	fakeDocker := tmpDir + "/docker"
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	wsDir := t.TempDir()
	opts := &tools.SandboxOpts{Level: tools.SandboxContainer}
	_, args, _ := tools.WrapWithSandbox("pwd", wsDir, opts)

	// Find the -v flag and verify workspace mount.
	found := false
	for i, a := range args {
		if a == "-v" && i+1 < len(args) {
			if strings.HasSuffix(args[i+1], ":/workspace") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("workspace mount not found in args: %v", args)
	}
}
