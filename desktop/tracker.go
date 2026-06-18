package desktop

import (
	"time"

	"github.com/jezek/xgb/xproto"

	"github.com/jezek/xgbutil"
	"github.com/jezek/xgbutil/xevent"
	"github.com/jezek/xgbutil/xprop"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

type Tracker struct {
	Clients     map[xproto.Window]*store.Client // List of tracked clients
	Workspaces  map[store.Location]*Workspace   // List of workspaces per location
	Channels    *Channels                       // Helper for channel communication
	Handlers    *Handlers                       // Helper for event handlers
	Initialized bool                            // Initial client restoration is complete

}
type Channels struct {
	Event  chan string // Channel for events
	Action chan string // Channel for actions
}

type Handlers struct {
	Timer        *time.Timer // Timer to handle delayed structure events
	ResizeClient *Handler    // Stores client for proportion change
	MoveClient   *Handler    // Stores client for tiling after move
	SwapClient   *Handler    // Stores clients for window swap
	SwapScreen   *Handler    // Stores client for screen swap
	InsertClient *Handler    // Stores clients + edge for split-insert
}

func (h *Handlers) Active() bool {
	return h.ResizeClient.Active() || h.MoveClient.Active() || h.SwapClient.Active() || h.SwapScreen.Active() || h.InsertClient.Active()
}

func (h *Handlers) Reset() {
	h.ResizeClient.Reset()
	h.MoveClient.Reset()
	h.SwapClient.Reset()
	h.SwapScreen.Reset()
	h.InsertClient.Reset()
}

type Handler struct {
	Dragging bool        // Indicates pointer dragging event
	Source   interface{} // Stores moved/resized client
	Target   interface{} // Stores client/workspace
	Edge     string      // Optional edge hint for insert handler (left/right/top/bottom)
}

func (h *Handler) Active() bool {
	return h.Source != nil
}

func (h *Handler) Reset() {
	*h = Handler{}
}

func CreateTracker() *Tracker {
	tr := Tracker{
		Clients:    make(map[xproto.Window]*store.Client),
		Workspaces: CreateWorkspaces(),
		Channels: &Channels{
			Event:  make(chan string),
			Action: make(chan string),
		},
		Handlers: &Handlers{
			ResizeClient: &Handler{},
			MoveClient:   &Handler{},
			SwapClient:   &Handler{},
			SwapScreen:   &Handler{},
			InsertClient: &Handler{},
		},
		Initialized: false,
	}

	// Attach to root events
	store.OnStateUpdate(tr.onStateUpdate)
	store.OnPointerUpdate(tr.onPointerUpdate)

	return &tr
}

func (tr *Tracker) Update() {
	ws := tr.ActiveWorkspace()
	if ws.TilingDisabled() {
		return
	}
	log.Debug("Update trackable clients [", len(tr.Clients), "/", len(store.Windows.Stacked), "]")

	// Map trackable windows
	trackable := make(map[xproto.Window]bool)
	for _, w := range store.Windows.Stacked {
		info := store.GetInfo(w.Id)
		trackable[w.Id] = !store.IsSpecial(info) && !store.IsIgnored(info)
		if common.Config.WindowFloatingAbove && !trackable[w.Id] && store.IsFloating(info) {
			store.SetAbove(w.Id)
		}
	}

	// Remove untrackable windows
	for w := range tr.Clients {
		if !trackable[w] {
			c := tr.Clients[w]
			above := store.IsAbove(store.GetInfo(w))
			tr.untrackWindow(w)
			if above {
				log.Info("Centering newly-above window [", c.Latest.Class, "]")
				c.CenterOnScreen()
			}
		}
	}

	// Add trackable windows
	for _, w := range store.Windows.Stacked {
		if trackable[w.Id] {
			tr.trackWindow(w.Id)
		}
	}

	tr.Initialized = true
}

func (tr *Tracker) Reset() {
	log.Debug("Reset trackable clients [", len(tr.Clients), "/", len(store.Windows.Stacked), "]")

	// Reset client list
	for w := range tr.Clients {
		tr.untrackWindow(w)
	}

	// Reset workspaces
	tr.Workspaces = CreateWorkspaces()
	tr.Initialized = false

	// Communicate workplace change
	tr.Channels.Event <- "workplace_change"
}

func (tr *Tracker) Tile(ws *Workspace) {
	if ws.TilingDisabled() {
		return
	}

	// Tile workspace
	ws.Tile()

	// Communicate clients change
	tr.Channels.Event <- "clients_change"

	// Communicate workspaces change
	tr.Channels.Event <- "workspaces_change"
}

func (tr *Tracker) Restore(ws *Workspace, flag uint8) {

	// Restore workspace
	ws.Restore(flag)

	// Communicate clients change
	tr.Channels.Event <- "clients_change"

	// Communicate workspaces change
	tr.Channels.Event <- "workspaces_change"
}

func (tr *Tracker) ActiveWorkspace() *Workspace {
	if store.Workplace == nil {
		return nil
	}
	return tr.WorkspaceAt(store.Workplace.CurrentDesktop, store.Workplace.CurrentScreen)
}

func (tr *Tracker) ClientWorkspace(c *store.Client) *Workspace {
	if c == nil {
		return nil
	}
	return tr.WorkspaceAt(c.Latest.Location.Desktop, c.Latest.Location.Screen)
}

func (tr *Tracker) WorkspaceAt(desktop uint, screen uint) *Workspace {
	location := store.Location{Desktop: desktop, Screen: screen}

	// Validate workspace
	ws := tr.Workspaces[location]
	if ws == nil {
		log.Warn("Invalid workspace [workspace-", location.Desktop, "-", location.Screen, "]")
	}

	return ws
}

func (tr *Tracker) ClientAt(ws *Workspace, p common.Point) *store.Client {
	if ws == nil {
		return nil
	}

	// Check if point hovers visible client
	for _, c := range ws.VisibleClients() {
		if c == nil {
			continue
		}
		if common.IsInsideRect(p, c.Latest.Dimensions.Geometry) {
			return c
		}
	}

	return nil
}

func (tr *Tracker) ActiveClient() *store.Client {
	c, exists := tr.Clients[store.Windows.Active.Id]

	// Validate client
	if !exists {
		return nil
	}

	return c
}

func (tr *Tracker) RefreshActiveClient() *store.Client {
	if focused, ok := store.InputFocusGet(store.X); ok {
		if c := tr.ClientForWindow(focused); c != nil {
			store.ActiveWindowUpdate(c.Window)
			return c
		}
	}

	active := store.ActiveWindowGet(store.X)
	if c := tr.ClientForWindow(active); c != nil {
		store.ActiveWindowUpdate(c.Window)
		return c
	}

	return tr.ActiveClient()
}

func (tr *Tracker) ClientForWindow(window store.XWindow) *store.Client {
	if tr == nil {
		return nil
	}
	return tr.Clients[window.Id]
}

func (tr *Tracker) unlockClients() {
	ws := tr.ActiveWorkspace()
	if ws == nil {
		return
	}

	// Unlock clients
	mg := ws.ActiveLayout().GetManager()
	for _, c := range mg.Clients(store.Stacked) {
		if c == nil {
			continue
		}
		c.UnLock()
	}
}

func (tr *Tracker) trackWindow(w xproto.Window) bool {
	if tr.isTracked(w) {
		return false
	}

	// Client and workspace
	c := store.CreateClient(w)
	if tr.Initialized {
		screen := store.ScreenGet(store.Pointer.Position)
		c.Latest.Location.Screen = screen
	}
	ws := tr.ClientWorkspace(c)
	if ws == nil {
		return false
	}

	// Add new client
	tr.Clients[c.Window.Id] = c
	ws.AddClient(c)

	// Attach handlers
	tr.attachHandlers(c)
	tr.Tile(ws)

	return true
}

func (tr *Tracker) untrackWindow(w xproto.Window) bool {
	if !tr.isTracked(w) {
		return false
	}

	// Client and workspace
	c := tr.Clients[w]
	ws := tr.ClientWorkspace(c)
	if ws == nil {
		return false
	}

	// Detach events
	xevent.Detach(store.X, w)

	// Restore client
	c.Restore(store.Latest)

	// Remove client
	ws.RemoveClient(c)
	delete(tr.Clients, w)

	// Tile workspace
	tr.Tile(ws)

	return true
}

func (tr *Tracker) handleMaximizedClient(c *store.Client) {
	if !tr.isTracked(c.Window.Id) {
		return
	}

	// Client maximized
	if store.IsMaximized(store.GetInfo(c.Window.Id)) {
		ws := tr.ClientWorkspace(c)
		if ws.TilingDisabled() {
			return
		}
		log.Debug("Client maximized handler fired [", c.Latest.Class, "]")

		// Update client states
		c.Update()

		// Unmaximize window
		c.UnMaximize()

		// Toggle maximized layout. The window manager sends the same maximize
		// state request when the title-bar button is used to leave maximized
		// mode because bsptile removes the native maximized state itself.
		if !c.IsNew() {
			tr.Channels.Action <- "layout_maximized"
			store.ActiveWindowSet(store.X, c.Window)
		}
	}
}

func (tr *Tracker) handleMinimizedClient(c *store.Client) {
	if !tr.isTracked(c.Window.Id) {
		return
	}

	// Client minimized
	if store.IsMinimized(store.GetInfo(c.Window.Id)) {
		ws := tr.ClientWorkspace(c)
		if ws.TilingDisabled() {
			return
		}
		log.Debug("Client minimized handler fired [", c.Latest.Class, "]")

		// Untrack client
		tr.untrackWindow(c.Window.Id)
	}
}

func (tr *Tracker) handleResizeClient(c *store.Client) {
	ws := tr.ClientWorkspace(c)
	if ws.TilingDisabled() || !tr.isTracked(c.Window.Id) || store.IsMaximized(store.GetInfo(c.Window.Id)) {
		return
	}

	// Previous dimensions
	pGeom := c.Latest.Dimensions.Geometry
	px, py, pw, ph := pGeom.Pieces()

	// Current dimensions
	cGeom, err := c.Window.Instance.DecorGeometry()
	if err != nil {
		return
	}
	cx, cy, cw, ch := cGeom.Pieces()

	// Check size changes
	resized := cw != pw || ch != ph
	moved := (cx != px || cy != py) && (cw == pw && ch == ph)

	if resized && !moved && !tr.Handlers.MoveClient.Active() {
		pt := store.PointerUpdate(store.X)

		// Set client resize event
		if !c.IsNew() && !tr.Handlers.ResizeClient.Active() {
			tr.Handlers.ResizeClient = &Handler{Dragging: pt.Dragging(500), Source: c}
		}
		log.Debug("Client resize handler fired [", c.Latest.Class, "]")

		if tr.Handlers.ResizeClient.Dragging {

			// Set client resize lock
			if tr.Handlers.ResizeClient.Active() {
				tr.Handlers.ResizeClient.Source.(*store.Client).Lock()
				log.Debug("Client resize handler active [", c.Latest.Class, "]")
			}

			// Update proportions
			dir := &store.Directions{
				Top:    cy != py,
				Right:  cx+cw != px+pw,
				Bottom: cy+ch != py+ph,
				Left:   cx != px,
			}
			ws.ActiveLayout().UpdateProportions(c, dir, *common.CreateGeometry(cGeom))
		}

		// Tile workspace
		tr.Tile(ws)
	}
}

func (tr *Tracker) handleMoveClient(c *store.Client) {
	ws := tr.ClientWorkspace(c)
	if !tr.isTracked(c.Window.Id) || store.IsMaximized(store.GetInfo(c.Window.Id)) {
		return
	}

	// Previous dimensions
	pGeom := c.Latest.Dimensions.Geometry
	px, py, pw, ph := pGeom.Pieces()

	// Current dimensions
	cGeom, err := c.Window.Instance.DecorGeometry()
	if err != nil {
		return
	}
	cx, cy, cw, ch := cGeom.Pieces()

	// Check position changes
	moved := cx != px || cy != py
	resized := cw != pw || ch != ph

	if moved && !resized && !tr.Handlers.ResizeClient.Active() {
		pt := store.PointerUpdate(store.X)

		// Set client move event
		if !c.IsNew() && !tr.Handlers.MoveClient.Active() {
			tr.Handlers.MoveClient = &Handler{Dragging: pt.Dragging(500), Source: c}
		}
		log.Debug("Client move handler fired [", c.Latest.Class, "]")

		// Obtain targets based on dragging indicator
		targetPoint := *common.CreatePoint(cx, cy)
		if tr.Handlers.MoveClient.Dragging {
			targetPoint = pt.Position
		}
		targetDesktop := store.Workplace.CurrentDesktop
		targetScreen := store.ScreenGet(targetPoint)

		// Check if target point hovers another client (possibly on a different screen)
		tr.Handlers.SwapClient.Reset()
		tr.Handlers.InsertClient.Reset()
		targetWs := tr.WorkspaceAt(targetDesktop, targetScreen)
		if targetWs == nil {
			targetWs = ws
		}
		if co := tr.ClientAt(targetWs, targetPoint); co != nil && co != c {
			tx, ty, tw, th := co.OuterGeometry()
			edge := dropEdge(targetPoint.X, targetPoint.Y, tx, ty, tw, th, 0.25)
			if edge == "" {
				tr.Handlers.SwapClient = &Handler{Source: c, Target: co}
				log.Debug("Client swap handler active [", c.Latest.Class, "-", co.Latest.Class, "]")
				store.ShowDropIndicator(&store.DropZone{Target: co.Window.Id, X: tx, Y: ty, W: tw, H: th})
			} else {
				tr.Handlers.InsertClient = &Handler{Source: c, Target: co, Edge: edge}
				log.Debug("Client insert handler active [", c.Latest.Class, " ", edge, " of ", co.Latest.Class, "]")
				zx, zy, zw, zh := dropZoneRect(edge, tx, ty, tw, th)
				store.ShowDropIndicator(&store.DropZone{Target: co.Window.Id, Edge: edge, X: zx, Y: zy, W: zw, H: zh})
			}
		} else {
			store.HideDropIndicator()
		}

		// Check if target point moves to another screen
		tr.Handlers.SwapScreen.Reset()
		if c.Latest.Location.Screen != targetScreen {
			tr.Handlers.SwapScreen = &Handler{Source: c, Target: tr.WorkspaceAt(targetDesktop, targetScreen)}
			log.Debug("Screen swap handler active [", c.Latest.Class, "]")
		}
	}
}

func (tr *Tracker) handleSwapClient(h *Handler) {
	c, target := h.Source.(*store.Client), h.Target.(*store.Client)
	ws := tr.ClientWorkspace(c)
	if !tr.isTracked(c.Window.Id) {
		return
	}
	log.Debug("Client swap handler fired [", c.Latest.Class, "-", target.Latest.Class, "]")

	// Swap clients on same desktop and screen
	mg := ws.ActiveLayout().GetManager()
	mg.SwapClient(c, target)

	// Reset client swapping handler
	h.Reset()

	// Tile workspace
	tr.Tile(ws)
}

func (tr *Tracker) handleWorkspaceChange(h *Handler) {
	c, target := h.Source.(*store.Client), h.Target.(*Workspace)
	if !tr.isTracked(c.Window.Id) {
		return
	}
	log.Debug("Client workspace handler fired [", c.Latest.Class, "]")

	// Remove client from current workspace
	ws := tr.ClientWorkspace(c)
	ws.RemoveClient(c)

	// Tile current workspace
	if ws.TilingEnabled() {
		tr.Tile(ws)
	}

	// Update client desktop and screen
	if !tr.isTrackable(c.Window.Id) {
		return
	}
	c.Update()

	// Add client to new workspace
	ws = tr.ClientWorkspace(c)
	if tr.Handlers.SwapScreen.Active() && target.TilingEnabled() {
		ws = target
	}
	ws.AddClient(c)

	// Tile new workspace
	if ws.TilingEnabled() {
		tr.Tile(ws)
	} else {
		c.Restore(store.Latest)
	}

	// Reset screen swapping handler
	h.Reset()
}

func (tr *Tracker) onStateUpdate(state string, desktop uint, screen uint) {
	workplaceChanged := store.Workplace.DesktopCount*store.Workplace.ScreenCount != uint(len(tr.Workspaces))
	workspaceChanged := common.IsInList(state, []string{"_NET_CURRENT_DESKTOP"})

	viewportChanged := common.IsInList(state, []string{"_NET_NUMBER_OF_DESKTOPS", "_NET_DESKTOP_LAYOUT", "_NET_DESKTOP_GEOMETRY", "_NET_DESKTOP_VIEWPORT", "_NET_WORKAREA"})
	clientsChanged := common.IsInList(state, []string{"_NET_CLIENT_LIST_STACKING"})
	focusChanged := common.IsInList(state, []string{"_NET_ACTIVE_WINDOW"})

	if workplaceChanged {

		// Reset clients and workspaces
		tr.Reset()
	}

	if workspaceChanged {

		// Update sticky windows
		for _, c := range tr.Clients {
			if store.IsSticky(c.Latest) && c.Latest.Location.Desktop != store.Workplace.CurrentDesktop {
				c.MoveToDesktop(^uint32(0))
			}
		}
	}

	if viewportChanged || clientsChanged || focusChanged {

		// Deactivate handlers
		store.HideDropIndicator()
		tr.Handlers.Reset()

		// Unlock clients
		tr.unlockClients()

		// Update trackable clients
		tr.Update()
	}

	if focusChanged {
		// Maximized and fullscreen layouts follow the active window. Reapply
		// them after the window manager publishes the new active window.
		ws := tr.ClientWorkspace(tr.ActiveClient())
		if ws != nil && ws.ActiveWindowLayout() {
			tr.Tile(ws)
		}

		// Communicate windows change
		tr.Channels.Event <- "windows_change"
	}
}

func (tr *Tracker) onPointerUpdate(pointer store.XPointer, desktop uint, screen uint) {
	buttonReleased := !pointer.Pressed()

	// Reset timer
	if tr.Handlers.Timer != nil {
		tr.Handlers.Timer.Stop()
	}

	// Wait on button release
	var t time.Duration = 0
	if buttonReleased {
		t = 50
	}

	// Wait for structure events
	tr.Handlers.Timer = time.AfterFunc(t*time.Millisecond, func() {

		// Window moved to another screen
		if tr.Handlers.SwapScreen.Active() {
			tr.handleWorkspaceChange(tr.Handlers.SwapScreen)
		}

		// Window moved over another window
		if tr.Handlers.SwapClient.Active() {
			tr.handleSwapClient(tr.Handlers.SwapClient)
		}
		if tr.Handlers.InsertClient.Active() {
			tr.handleInsertClient(tr.Handlers.InsertClient)
		}

		// Window moved or resized
		if tr.Handlers.MoveClient.Active() || tr.Handlers.ResizeClient.Active() {
			store.HideDropIndicator()
			tr.Handlers.MoveClient.Reset()
			tr.Handlers.ResizeClient.Reset()

			// Unlock clients
			tr.unlockClients()

			// Tile workspace
			if buttonReleased {
				tr.Tile(tr.ActiveWorkspace())
			}
		}
	})
}

func (tr *Tracker) attachHandlers(c *store.Client) {
	c.Window.Instance.Listen(xproto.EventMaskStructureNotify | xproto.EventMaskPropertyChange | xproto.EventMaskFocusChange)

	// Track focus immediately instead of waiting for _NET_ACTIVE_WINDOW updates.
	xevent.FocusInFun(func(X *xgbutil.XUtil, ev xevent.FocusInEvent) {
		log.Trace("Client focus event [", c.Latest.Class, "]")
		store.ActiveWindowUpdate(c.Window)
	}).Connect(store.X, c.Window.Id)

	// Attach structure events
	xevent.ConfigureNotifyFun(func(X *xgbutil.XUtil, ev xevent.ConfigureNotifyEvent) {
		log.Trace("Client structure event [", c.Latest.Class, "]")

		// Handle structure events
		tr.handleResizeClient(c)
		tr.handleMoveClient(c)
		if !tr.Handlers.MoveClient.Active() {
			c.Update()
		}
	}).Connect(store.X, c.Window.Id)

	// Attach property events
	xevent.PropertyNotifyFun(func(X *xgbutil.XUtil, ev xevent.PropertyNotifyEvent) {
		aname, _ := xprop.AtomName(store.X, ev.Atom)
		log.Trace("Client property event ", aname, " [", c.Latest.Class, "]")

		// Handle property events
		if aname == "_NET_WM_STATE" {
			tr.handleMaximizedClient(c)
			tr.handleMinimizedClient(c)
		} else if aname == "_NET_WM_DESKTOP" {
			tr.handleWorkspaceChange(&Handler{Source: c, Target: tr.ActiveWorkspace()})
		}
	}).Connect(store.X, c.Window.Id)
}

func (tr *Tracker) isTracked(w xproto.Window) bool {
	_, ok := tr.Clients[w]
	return ok
}

func (tr *Tracker) isTrackable(w xproto.Window) bool {
	info := store.GetInfo(w)
	return !store.IsSpecial(info) && !store.IsIgnored(info)
}

func dropEdge(px, py, x, y, w, h int, zone float64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	rx := float64(px-x) / float64(w)
	ry := float64(py-y) / float64(h)
	dl, dr, dt, db := rx, 1-rx, ry, 1-ry
	min := dl
	edge := "left"
	if dr < min {
		min, edge = dr, "right"
	}
	if dt < min {
		min, edge = dt, "top"
	}
	if db < min {
		min, edge = db, "bottom"
	}
	if min > zone {
		return ""
	}
	return edge
}

func dropZoneRect(edge string, x, y, w, h int) (int, int, int, int) {
	switch edge {
	case "left":
		return x, y, w / 2, h
	case "right":
		return x + w/2, y, w - w/2, h
	case "top":
		return x, y, w, h / 2
	case "bottom":
		return x, y + h/2, w, h - h/2
	}
	return x, y, w, h
}

func (tr *Tracker) handleInsertClient(h *Handler) {
	c, target := h.Source.(*store.Client), h.Target.(*store.Client)
	ws := tr.ClientWorkspace(c)
	if !tr.isTracked(c.Window.Id) || ws == nil {
		return
	}
	log.Debug("Client insert handler fired [", c.Latest.Class, " ", h.Edge, " of ", target.Latest.Class, "]")
	mg := ws.ActiveLayout().GetManager()
	mg.InsertClient(c, target, h.Edge)
	h.Reset()
	tr.Tile(ws)
}
