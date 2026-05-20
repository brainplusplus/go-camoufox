package virtdisplay

import (
	"errors"
	"runtime"
	"testing"
)

func TestApplyEnv(t *testing.T) {
	env := map[string]string{"WAYLAND_DISPLAY": "wayland-0"}
	ApplyEnv(env, ":99")
	if env["DISPLAY"] != ":99" || env["GDK_BACKEND"] != "x11" || env["MOZ_ENABLE_WAYLAND"] != "0" {
		t.Fatalf("virtual display env mismatch: %#v", env)
	}
	if _, ok := env["WAYLAND_DISPLAY"]; ok {
		t.Fatalf("WAYLAND_DISPLAY should be removed: %#v", env)
	}
}

func TestNewVirtualDisplayUnsupportedOutsideLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux behavior test")
	}
	_, err := New("", false)
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}
