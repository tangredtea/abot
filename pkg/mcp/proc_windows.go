//go:build windows

package mcp

import "os/exec"

func setMCPProcGroup(_ *exec.Cmd) {}
