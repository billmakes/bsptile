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

func TestFrameExtentsClampsGtkSubtraction(t *testing.T) {
	got := FrameExtents([]uint{1, 2, 3, 4}, []uint{3, 1, 10, 2})
	if got.Left != 0 || got.Right != 1 || got.Top != 0 || got.Bottom != 2 {
		t.Fatalf("FrameExtents = %+v, want clamped signed subtraction", got)
	}
}

func TestFrameExtentsIgnoresExtraValues(t *testing.T) {
	got := FrameExtents([]uint{1, 2, 3, 4, 99}, []uint{0, 0, 0, 0, 99})
	if got.Left != 1 || got.Right != 2 || got.Top != 3 || got.Bottom != 4 {
		t.Fatalf("FrameExtents = %+v, want first four values only", got)
	}
}
