//go:build windows

package bidi

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func terminateProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	killTree := exec.Command("taskkill", "/PID", pid, "/T", "/F")
	output, err := killTree.CombinedOutput()
	if err == nil {
		return nil
	}
	text := strings.ToLower(string(output))
	if strings.Contains(text, "not found") || strings.Contains(text, "not running") {
		return nil
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
