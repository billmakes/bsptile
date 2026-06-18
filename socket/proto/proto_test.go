package proto

import (
	"os"
	"strings"
	"testing"
)

func TestSocketPathPrefersExplicitEnv(t *testing.T) {
	t.Setenv("BSPTILE_SOCKET", "/tmp/explicit.sock")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("DISPLAY", ":1")

	if got := SocketPath(); got != "/tmp/explicit.sock" {
		t.Fatalf("SocketPath() = %q, want /tmp/explicit.sock", got)
	}
}

func TestSocketPathFallsBackToXDGRuntimeDir(t *testing.T) {
	os.Unsetenv("BSPTILE_SOCKET")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("DISPLAY", ":2")

	got := SocketPath()
	if got != "/run/user/1000/bsptile-2.sock" {
		t.Fatalf("SocketPath() = %q, want /run/user/1000/bsptile-2.sock", got)
	}
}

func TestSocketPathFallsBackToTmp(t *testing.T) {
	os.Unsetenv("BSPTILE_SOCKET")
	os.Unsetenv("XDG_RUNTIME_DIR")
	t.Setenv("DISPLAY", ":3")

	got := SocketPath()
	if !strings.HasPrefix(got, "/tmp/bsptile-3-") || !strings.HasSuffix(got, ".sock") {
		t.Fatalf("SocketPath() = %q, want /tmp/bsptile-3-<uid>.sock", got)
	}
}

func TestSocketPathDefaultsDisplayWhenUnset(t *testing.T) {
	os.Unsetenv("BSPTILE_SOCKET")
	os.Unsetenv("DISPLAY")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	got := SocketPath()
	if got != "/run/user/1000/bsptile-0.sock" {
		t.Fatalf("SocketPath() = %q, want /run/user/1000/bsptile-0.sock", got)
	}
}
