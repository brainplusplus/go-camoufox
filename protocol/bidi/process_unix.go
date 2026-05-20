//go:build !windows

package bidi

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func terminateProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(2 * time.Second)
	return syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
}
