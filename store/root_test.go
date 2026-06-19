package store

import (
	"testing"

	"github.com/billmakes/bsptile/v2/common"
)

func TestClampDesktopToScreenKeepsRightMonitorInsideBounds(t *testing.T) {
	screen := common.Geometry{X: 1920, Y: 0, Width: 1920, Height: 1080}
	desktop := common.Geometry{X: 1823, Y: 0, Width: 1917, Height: 1043}

	got := clampDesktopToScreen(desktop, screen)
	want := common.Geometry{X: 1920, Y: 0, Width: 1820, Height: 1043}
	if got != want {
		t.Fatalf("clamped desktop = %+v, want %+v", got, want)
	}
}

func TestClampDesktopToScreenPreservesRightEdgeStrut(t *testing.T) {
	screen := common.Geometry{X: 1920, Y: 0, Width: 1920, Height: 1080}
	desktop := common.Geometry{X: 0, Y: 0, Width: 3772, Height: 1043}

	got := clampDesktopToScreen(desktop, screen)
	want := common.Geometry{X: 1920, Y: 0, Width: 1852, Height: 1043}
	if got != want {
		t.Fatalf("clamped desktop = %+v, want %+v", got, want)
	}
}

func TestClampDesktopToScreenFallsBackForInvalidDesktop(t *testing.T) {
	screen := common.Geometry{X: 1920, Y: 0, Width: 1920, Height: 1080}
	desktop := common.Geometry{X: 1823, Y: 0, Width: 0, Height: -10}

	if got := clampDesktopToScreen(desktop, screen); got != screen {
		t.Fatalf("clamped desktop = %+v, want screen %+v", got, screen)
	}
}
