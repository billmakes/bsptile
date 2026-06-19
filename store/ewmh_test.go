package store

import (
	"testing"

	"github.com/jezek/xgbutil/ewmh"
)

func TestWindowStateActionName(t *testing.T) {
	tests := map[int]string{
		ewmh.StateRemove: "remove",
		ewmh.StateAdd:    "add",
		ewmh.StateToggle: "toggle",
		99:               "99",
	}
	for action, want := range tests {
		if got := WindowStateActionName(action); got != want {
			t.Fatalf("action %d name = %q, want %q", action, got, want)
		}
	}
}
