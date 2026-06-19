package input

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
	"github.com/billmakes/bsptile/v2/layout"
	"github.com/billmakes/bsptile/v2/store"
)

func directionTestClient(id xproto.Window, geometry common.Geometry) *store.Client {
	return &store.Client{
		Window: &store.XWindow{Id: id},
		Latest: &store.Info{
			Class: "test",
			Dimensions: store.Dimensions{
				Geometry: geometry,
			},
		},
	}
}

func TestDirectionWorkspaceClientPrioritizesDistanceInRequestedDirection(t *testing.T) {
	source := directionTestClient(1, common.Geometry{X: 2000, Y: 400, Width: 400, Height: 400})
	nearLeft := directionTestClient(2, common.Geometry{X: 1400, Y: 200, Width: 400, Height: 400})
	farAlignedLeft := directionTestClient(3, common.Geometry{X: 0, Y: 400, Width: 400, Height: 400})

	manager := store.CreateBSPManager(store.Location{})
	manager.AddClient(farAlignedLeft)
	store.Windows = &store.XWindows{Active: *farAlignedLeft.Window}
	manager.AddClient(nearLeft)
	target := &desktop.Workspace{
		Layouts: []desktop.Layout{layout.CreateBSPLayout(manager)},
	}

	if selected := DirectionWorkspaceClient(source, target, common.Left); selected != nearLeft {
		t.Fatalf("left target = %v, want nearest window", selected)
	}
}

func TestDirectionWorkspaceClientPrefersPerpendicularOverlap(t *testing.T) {
	source := directionTestClient(1, common.Geometry{X: 1000, Y: 500, Width: 400, Height: 400})
	directAbove := directionTestClient(2, common.Geometry{X: 1000, Y: 0, Width: 400, Height: 400})
	closerDiagonal := directionTestClient(3, common.Geometry{X: 500, Y: 350, Width: 400, Height: 100})

	manager := store.CreateBSPManager(store.Location{})
	manager.AddClient(directAbove)
	store.Windows = &store.XWindows{Active: *directAbove.Window}
	manager.AddClient(closerDiagonal)
	target := &desktop.Workspace{
		Layouts: []desktop.Layout{layout.CreateBSPLayout(manager)},
	}

	if selected := DirectionWorkspaceClient(source, target, common.Up); selected != directAbove {
		t.Fatalf("up target = %v, want directly overlapping window", selected)
	}
}

func TestDirectionWorkspaceClientPrefersNearbyDiagonalOverFarAligned(t *testing.T) {
	source := directionTestClient(1, common.Geometry{X: 1000, Y: 500, Width: 400, Height: 400})
	nearDiagonal := directionTestClient(2, common.Geometry{X: 500, Y: 100, Width: 400, Height: 300})
	farAligned := directionTestClient(3, common.Geometry{X: -1000, Y: 500, Width: 400, Height: 400})

	manager := store.CreateBSPManager(store.Location{})
	manager.AddClient(farAligned)
	store.Windows = &store.XWindows{Active: *farAligned.Window}
	manager.AddClient(nearDiagonal)
	target := &desktop.Workspace{
		Layouts: []desktop.Layout{layout.CreateBSPLayout(manager)},
	}

	if selected := DirectionWorkspaceClient(source, target, common.Left); selected != nearDiagonal {
		t.Fatalf("left target = %v, want nearby diagonal window", selected)
	}
}

func TestShouldWarpPointerRequiresConfigAndPointerInsideClient(t *testing.T) {
	previousConfig := common.Config.WindowPointerWarp
	previousPointer := store.Pointer
	t.Cleanup(func() {
		common.Config.WindowPointerWarp = previousConfig
		store.Pointer = previousPointer
	})

	client := directionTestClient(1, common.Geometry{X: 100, Y: 100, Width: 400, Height: 400})
	store.Pointer = &store.XPointer{Position: common.Point{X: 200, Y: 200}}

	common.Config.WindowPointerWarp = false
	if shouldWarpPointer(client) {
		t.Fatal("pointer warp enabled while configuration is false")
	}

	common.Config.WindowPointerWarp = true
	if !shouldWarpPointer(client) {
		t.Fatal("pointer warp disabled while configured and pointer is inside client")
	}

	store.Pointer.Position = common.Point{X: 0, Y: 0}
	if shouldWarpPointer(client) {
		t.Fatal("pointer warp enabled while pointer is outside client")
	}
}

func TestHoverFocusUsesOnlyManagedTopmostWindow(t *testing.T) {
	managed := directionTestClient(1, common.Geometry{})
	tracker := &desktop.Tracker{
		Clients: map[xproto.Window]*store.Client{
			managed.Window.Id: managed,
		},
	}

	if client := hoverClient(tracker, *managed.Window); client != managed {
		t.Fatal("hover focus rejected managed topmost window")
	}
	if client := hoverClient(tracker, store.XWindow{Id: 2}); client != nil {
		t.Fatal("hover focus accepted unmanaged topmost window")
	}
}

func TestSwitchDesktopRejectsInvalidTargetsWithoutX(t *testing.T) {
	previousWorkplace := store.Workplace
	previousX := store.X
	t.Cleanup(func() {
		store.Workplace = previousWorkplace
		store.X = previousX
	})

	store.X = nil
	store.Workplace = &store.XWorkplace{DesktopCount: 2, CurrentDesktop: 0}

	if SwitchDesktop(2) {
		t.Fatal("out-of-range desktop switch succeeded")
	}
	if SwitchDesktop(0) {
		t.Fatal("same-desktop switch succeeded")
	}
	if ok, handled := tryNumberedAction("desktop_0", nil, nil); !handled || ok {
		t.Fatalf("desktop_0 = ok %v handled %v, want handled failure", ok, handled)
	}
	if ok, handled := tryNumberedAction("desktop_x", nil, nil); !handled || ok {
		t.Fatalf("desktop_x = ok %v handled %v, want handled failure", ok, handled)
	}
	if _, handled := tryNumberedAction("not_desktop_1", nil, nil); handled {
		t.Fatal("unrelated action was handled as numbered desktop action")
	}
}

func TestActiveActionWindowFallsBackToCachedActiveWindow(t *testing.T) {
	previousX := store.X
	previousWindows := store.Windows
	t.Cleanup(func() {
		store.X = previousX
		store.Windows = previousWindows
	})

	store.X = nil
	store.Windows = &store.XWindows{Active: store.XWindow{Id: 42}}

	w := activeActionWindow(nil)
	if w == nil || w.Id != 42 {
		t.Fatalf("activeActionWindow = %+v, want cached active window 42", w)
	}
}
