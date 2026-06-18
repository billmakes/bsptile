package input

import (
	"sync"
	"time"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
	"github.com/billmakes/bsptile/v2/store"
	"github.com/billmakes/bsptile/v2/ui"

	log "github.com/sirupsen/logrus"
)

var (
	workspace *desktop.Workspace // Stores previous workspace (for comparison only)
	pointer   *store.XPointer    // Stores previous pointer (for comparison only)
	hover     *time.Timer        // Timer to delay hover events
	hoverLock sync.Mutex         // Protects hover timer state
	hoverID   uint64             // Invalidates stale hover callbacks
)

func BindMouse(tr *desktop.Tracker) {
	poll(100, func() {
		store.PointerUpdate(store.X)

		// Reset tracker handler
		resetTracker(tr)

		// Evaluate workspace state
		updateWorkspace(tr)

		// Evaluate corner state
		updateCorner(tr)

		// Evaluate focus state
		updateFocus(tr)

		// Store last pointer
		pointer = store.Pointer
	})
}

func resetTracker(tr *desktop.Tracker) {
	if pointer == nil || pointer.Position != store.Pointer.Position {
		return
	}

	// Reset tracker handler
	if !tr.Handlers.MoveClient.Active() {
		tr.Handlers.Reset()
	}
}

func updateWorkspace(tr *desktop.Tracker) {
	ws := tr.ActiveWorkspace()
	if ws == nil || ws == workspace {
		return
	}
	log.Info("Active workspace updated [", ws.Name, "]")

	// Communicate workplace change
	tr.Channels.Event <- "workplace_change"

	// Update systray icon
	ui.UpdateIcon(ws)

	// Store last workspace
	workspace = ws
}

func updateCorner(tr *desktop.Tracker) {
	hc := store.HotCorner()
	if hc == nil {
		return
	}

	// Communicate corner change
	tr.Channels.Event <- "corner_change"

	// Execute action
	ExecuteAction(common.Config.Corners[hc.Name], tr, tr.ActiveWorkspace())
}

func updateFocus(tr *desktop.Tracker) {
	ws := tr.ActiveWorkspace()
	if ws == nil || pointer == nil {
		return
	}

	// Ignore stationary pointer position
	if pointer.Position == store.Pointer.Position {
		return
	}

	// Ignore untracked clients
	hovered := tr.ClientAt(ws, store.Pointer.Position)
	if hovered == nil {
		return
	}
	log.Info("Hovered window updated [", hovered.Latest.Class, "]")

	// Delay hover event by given duration
	if common.Config.WindowFocusDelay <= 0 {
		return
	}
	hoverLock.Lock()
	if hover != nil {
		hover.Stop()
	}
	hoverID++
	id := hoverID
	hover = time.AfterFunc(time.Duration(common.Config.WindowFocusDelay)*time.Millisecond, func() {
		hoverLock.Lock()
		if id != hoverID {
			hoverLock.Unlock()
			return
		}
		hover = nil
		hoverLock.Unlock()

		// Hovered client window has changed in the meantime
		if hovered != tr.ClientAt(ws, store.Pointer.Position) {
			return
		}

		// Focus hovered client window
		active := tr.ActiveClient()
		if hovered != active && ws.TilingEnabled() && !tr.Handlers.Active() {
			store.ActiveWindowSet(store.X, hovered.Window)
		}
	})
	hoverLock.Unlock()
}

func cancelHoverFocus() {
	hoverLock.Lock()
	defer hoverLock.Unlock()

	hoverID++
	if hover != nil {
		hover.Stop()
		hover = nil
	}
}

func poll(t time.Duration, fun func()) {
	go func() {
		for range time.Tick(t * time.Millisecond) {
			fun()
		}
	}()
}
