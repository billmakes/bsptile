package layout

import (
	"testing"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"
)

func TestMaximizedGeometryAppliesGapOnEveryEdge(t *testing.T) {
	desktop := common.Geometry{X: 100, Y: 50, Width: 1200, Height: 700}
	got := MaximizedGeometry(desktop, 10)
	want := common.Geometry{X: 110, Y: 60, Width: 1180, Height: 680}
	if got != want {
		t.Fatalf("maximized geometry = %+v, want %+v", got, want)
	}
}

func TestMaximizedGeometryClampsSmallDimensions(t *testing.T) {
	got := MaximizedGeometry(common.Geometry{Width: 10, Height: 8}, 20)
	if got.Width != 1 || got.Height != 1 {
		t.Fatalf("clamped geometry = %+v, want 1x1", got)
	}
}

func TestFullscreenGeometryUsesEntireScreen(t *testing.T) {
	screen := common.Geometry{X: 1920, Y: 0, Width: 2560, Height: 1440}
	if got := FullscreenGeometry(screen); got != screen {
		t.Fatalf("fullscreen geometry = %+v, want %+v", got, screen)
	}
}

func TestLayoutConstructorsShareManagerAndExposeNames(t *testing.T) {
	manager := store.CreateBSPManager(store.Location{Desktop: 2, Screen: 1})
	layouts := []struct {
		name string
		got  interface {
			GetName() string
			GetManager() *store.Manager
		}
	}{
		{name: "bsp", got: CreateBSPLayout(manager)},
		{name: "maximized", got: CreateMaximizedLayout(manager)},
		{name: "fullscreen", got: CreateFullscreenLayout(manager)},
	}

	for _, test := range layouts {
		if test.got.GetName() != test.name {
			t.Fatalf("layout name = %q, want %q", test.got.GetName(), test.name)
		}
		if test.got.GetManager() != manager {
			t.Fatalf("%s layout did not retain shared manager", test.name)
		}
	}
}

func TestNonBSPProportionUpdatesResetTree(t *testing.T) {
	manager := store.CreateBSPManager(store.Location{})
	first := &store.Client{Window: &store.XWindow{Id: 1}, Latest: &store.Info{Class: "first"}}
	second := &store.Client{Window: &store.XWindow{Id: 2}, Latest: &store.Info{Class: "second"}}
	manager.AddClient(first)
	store.Windows = &store.XWindows{Active: *first.Window}
	manager.AddClient(second)
	manager.Root.Ratio = 0.8

	CreateMaximizedLayout(manager).UpdateProportions(nil, nil, common.Geometry{})
	if manager.Root.Ratio != 0.5 {
		t.Fatalf("maximized reset ratio = %v, want 0.5", manager.Root.Ratio)
	}

	manager.Root.Ratio = 0.8
	CreateFullscreenLayout(manager).UpdateProportions(nil, nil, common.Geometry{})
	if manager.Root.Ratio != 0.5 {
		t.Fatalf("fullscreen reset ratio = %v, want 0.5", manager.Root.Ratio)
	}
}
