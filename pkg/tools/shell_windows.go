//go:build windows

package tools

import "os/exec"

// setProcGroup is a no-op on Windows.
func setProcGroup(_ *exec.Cmd) {}
