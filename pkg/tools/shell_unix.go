//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in its own process group.
// On timeout, Go kills the entire group instead of just the parent process.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
}
