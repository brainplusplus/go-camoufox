package virtdisplay

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const DisplayFDReadTimeout = 10 * time.Second

var xvfbArgs = []string{
	"-screen", "0", "1x1x24",
	"-ac",
	"-nolisten", "tcp",
	"-extension", "RENDER",
	"+extension", "GLX",
	"-extension", "COMPOSITE",
	"-extension", "XVideo",
	"-extension", "XVideo-MotionCompensation",
	"-extension", "XINERAMA",
	"-fp", "built-ins",
	"-nocursor",
	"-br",
}

var (
	ErrNotSupported  = errors.New("virtual display is only supported on Linux")
	ErrCannotFind    = errors.New("cannot find Xvfb")
	ErrCannotExecute = errors.New("cannot execute Xvfb")
)

type VirtualDisplay struct {
	Display string
	cmd     *exec.Cmd
}

func New(display string, debug bool) (*VirtualDisplay, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrNotSupported
	}
	if display != "" {
		return &VirtualDisplay{Display: display}, nil
	}
	path, err := exec.LookPath("Xvfb")
	if err != nil {
		return nil, fmt.Errorf("%w: please install Xvfb to use headless virtual mode", ErrCannotFind)
	}
	args := append([]string{"-displayfd", "3"}, xvfbArgs...)
	readFile, writeFile, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer readFile.Close()
	cmd := exec.Command(path, args...)
	cmd.Stdin = nil
	if !debug {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	cmd.ExtraFiles = []*os.File{writeFile}
	cmd.Env = append(os.Environ(), "__GLX_VENDOR_LIBRARY_NAME=mesa", "LIBGL_ALWAYS_SOFTWARE=1")
	if err := cmd.Start(); err != nil {
		_ = writeFile.Close()
		return nil, fmt.Errorf("%w: %v", ErrCannotExecute, err)
	}
	_ = writeFile.Close()

	displayCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := readFile.Read(buf)
		if err != nil {
			errCh <- err
			return
		}
		displayCh <- strings.TrimSpace(string(buf[:n]))
	}()

	var displayNum string
	select {
	case displayNum = <-displayCh:
	case err := <-errCh:
		_ = kill(cmd)
		return nil, fmt.Errorf("%w: Xvfb did not report a display: %v", ErrCannotExecute, err)
	case <-time.After(DisplayFDReadTimeout):
		_ = kill(cmd)
		return nil, fmt.Errorf("%w: Xvfb did not report a display within %s", ErrCannotExecute, DisplayFDReadTimeout)
	}
	if _, err := strconv.Atoi(displayNum); err != nil {
		_ = kill(cmd)
		return nil, fmt.Errorf("%w: Xvfb wrote non-integer display: %q", ErrCannotExecute, displayNum)
	}
	return &VirtualDisplay{Display: ":" + displayNum, cmd: cmd}, nil
}

func ApplyEnv(env map[string]string, display string) {
	env["DISPLAY"] = display
	env["GDK_BACKEND"] = "x11"
	delete(env, "WAYLAND_DISPLAY")
	env["MOZ_ENABLE_WAYLAND"] = "0"
}

func (vd *VirtualDisplay) Close() error {
	if vd == nil || vd.cmd == nil || vd.cmd.Process == nil {
		return nil
	}
	return kill(vd.cmd)
}

func kill(cmd *exec.Cmd) error {
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	_, err := cmd.Process.Wait()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
