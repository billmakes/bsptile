package input

import (
	"os"
	"strings"
	"syscall"
	"time"

	"os/exec"

	"github.com/jezek/xgbutil/xevent"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
	"github.com/billmakes/bsptile/v2/store"
	"github.com/billmakes/bsptile/v2/ui"

	log "github.com/sirupsen/logrus"
)

var (
	executeCallbacksFun []func(string, uint, uint) // Execute events callback functions
)

func Bind(tr *desktop.Tracker) {
	BindSignal(tr)
	BindMouse(tr)
	BindKeys(tr)
	BindTray(tr)
	BindDbus(tr)
	BindAddons(tr)
}

func ExecuteAction(action string, tr *desktop.Tracker, ws *desktop.Workspace) bool {
	success := false
	if len(action) == 0 || tr == nil || ws == nil {
		return false
	}
	cancelHoverFocus()

	log.Info("Execute action ", action, " [", ws.Name, "]")

	// Choose action command
	if mode, ok := keyModeAction(action); ok {
		success = SetKeyMode(mode, tr)
		goto complete
	}
	switch action {
	case "enable":
		success = EnableTiling(tr, ws)
	case "disable":
		success = DisableTiling(tr, ws)
	case "toggle":
		success = ToggleTiling(tr, ws)
	case "decoration":
		success = ToggleDecoration(tr, ws)
	case "restore":
		success = Restore(tr, ws)
	case "reset":
		success = Reset(tr, ws)
	case "balance":
		success = Balance(tr, ws)
	case "tree_rotate":
		success = RotateTree(tr, ws)
	case "layout_bsp":
		success = BSPLayout(tr, ws)
	case "layout_maximized":
		success = MaximizedLayout(tr, ws)
	case "layout_fullscreen":
		success = FullscreenLayout(tr, ws)
	case "window_next":
		success = NextWindow(tr, ws)
	case "window_previous":
		success = PreviousWindow(tr, ws)
	case "window_up":
		success = DirectionWindow(tr, ws, common.Up)
	case "window_down":
		success = DirectionWindow(tr, ws, common.Down)
	case "window_left":
		success = DirectionWindow(tr, ws, common.Left)
	case "window_right":
		success = DirectionWindow(tr, ws, common.Right)
	case "move_window_up":
		success = MoveDirectionWindow(tr, ws, common.Up)
	case "move_window_down":
		success = MoveDirectionWindow(tr, ws, common.Down)
	case "move_window_left":
		success = MoveDirectionWindow(tr, ws, common.Left)
	case "move_window_right":
		success = MoveDirectionWindow(tr, ws, common.Right)
	case "position_next":
		success = NextPosition(tr, ws)
	case "position_previous":
		success = PreviousPosition(tr, ws)
	case "screen_next":
		success = NextScreen(tr, ws)
	case "screen_previous":
		success = PreviousScreen(tr, ws)
	case "proportion_increase":
		success = IncreaseProportion(tr, ws)
	case "proportion_decrease":
		success = DecreaseProportion(tr, ws)
	case "proportion_up":
		success = DirectionProportion(tr, ws, common.Up)
	case "proportion_down":
		success = DirectionProportion(tr, ws, common.Down)
	case "proportion_left":
		success = DirectionProportion(tr, ws, common.Left)
	case "proportion_right":
		success = DirectionProportion(tr, ws, common.Right)
	case "reload":
		success = ReloadConfig(tr)
	case "restart":
		success = Restart(tr)
	case "exit":
		success = Exit(tr)
	default:
		success = External(action)
	}

complete:
	time.AfterFunc(100*time.Millisecond, tr.Handlers.Reset)

	// Check success
	if !success {
		return false
	}

	// Execute callbacks
	executeCallbacks(action, ws.Location.Desktop, ws.Location.Screen)

	return true
}

func ExecuteActions(action string, tr *desktop.Tracker, mod string) bool {
	client := tr.ClientWorkspace(tr.RefreshActiveClient())
	active := tr.ActiveWorkspace()

	// Use active client workspace as current
	if client != nil {
		active = client
	}

	// Execute actions per workspace
	results := []bool{}
	for _, ws := range tr.Workspaces {

		// Execute only on active screen
		if mod == "current" && ws.Location != active.Location {
			continue
		}

		// Execute only on active workspace
		if mod == "screens" && (ws.Location.Desktop != active.Location.Desktop) {
			continue
		}

		// Execute action and store results
		success := ExecuteAction(action, tr, ws)
		results = append(results, success)
	}

	return common.AllTrue(results)
}

func EnableTiling(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	ws.EnableTiling()
	tr.Update()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func DisableTiling(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.DisableTiling()
	tr.Restore(ws, store.Latest)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func ToggleTiling(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return EnableTiling(tr, ws)
	}
	return DisableTiling(tr, ws)
}

func EnableDecoration(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	mg := ws.ActiveLayout().GetManager()

	mg.EnableDecoration()
	tr.Update()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func DisableDecoration(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	mg := ws.ActiveLayout().GetManager()

	mg.DisableDecoration()
	tr.Update()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func ToggleDecoration(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	mg := ws.ActiveLayout().GetManager()
	if mg.DecorationDisabled() {
		return EnableDecoration(tr, ws)
	}
	return DisableDecoration(tr, ws)
}

func Restore(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.DisableTiling()
	tr.Restore(ws, store.Original)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func Reset(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.ResetLayouts()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func Balance(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.ActiveLayout().GetManager().Balance()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func RotateTree(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}

	ws.ActiveLayout().Rotate()
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func BSPLayout(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	for i, l := range ws.Layouts {
		if l.GetName() == "bsp" {
			ws.SetLayout(uint(i))
		}
	}
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func MaximizedLayout(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	target := "maximized"
	if ws.ActiveLayout().GetName() == "maximized" {
		target = "bsp"
	}
	for i, l := range ws.Layouts {
		if l.GetName() == target {
			ws.SetLayout(uint(i))
		}
	}
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func FullscreenLayout(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	target := "fullscreen"
	if ws.ActiveLayout().GetName() == "fullscreen" {
		target = "bsp"
	}
	for i, l := range ws.Layouts {
		if l.GetName() == target {
			ws.SetLayout(uint(i))
		}
	}
	tr.Tile(ws)

	ui.ShowLayout(ws)
	ui.UpdateIcon(ws)

	return true
}

func NextWindow(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	c := ws.ActiveLayout().NextClient()
	if c == nil {
		return false
	}

	store.ActiveWindowSet(store.X, c.Window)

	return true
}

func PreviousWindow(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	c := ws.ActiveLayout().PreviousClient()
	if c == nil {
		return false
	}

	store.ActiveWindowSet(store.X, c.Window)

	return true
}

func DirectionWindow(tr *desktop.Tracker, ws *desktop.Workspace, d common.Direction) bool {
	if ws.TilingDisabled() {
		return false
	}

	active := ws.ActiveLayout().ActiveClient()
	movePointer := shouldWarpPointer(active)

	c := ws.ActiveLayout().DirectionClient(d)
	if c == nil {
		screen, ok := DirectionScreen(ws.Location.Screen, d)
		if !ok {
			return false
		}
		target := tr.WorkspaceAt(ws.Location.Desktop, screen)
		c = DirectionWorkspaceClient(active, target, d)
		if c == nil {
			return false
		}
	}

	store.ActiveWindowSet(store.X, c.Window)
	if movePointer {
		store.PointerMove(store.X, c.Latest.Dimensions.Geometry.Center())
	}

	return true
}

func DirectionWorkspaceClient(source *store.Client, target *desktop.Workspace, d common.Direction) *store.Client {
	if source == nil || target == nil {
		return nil
	}

	var selected *store.Client
	var best common.DirectionScore

	for _, c := range target.VisibleClients() {
		if c == nil {
			continue
		}

		score, ok := common.ScoreDirection(source.Latest.Dimensions.Geometry, c.Latest.Dimensions.Geometry, d)
		if !ok {
			continue
		}
		if selected == nil || common.BetterDirectionScore(score, best) {
			selected = c
			best = score
		}
	}

	return selected
}

func MoveDirectionWindow(tr *desktop.Tracker, ws *desktop.Workspace, d common.Direction) bool {
	if ws.TilingDisabled() {
		return false
	}

	active := ws.ActiveLayout().ActiveClient()
	if active == nil {
		return false
	}
	movePointer := shouldWarpPointer(active)

	target := ws.ActiveLayout().DirectionClient(d)
	if target == nil {
		return moveDirectionAcrossScreens(tr, ws, active, d, movePointer)
	}
	targetCenter := target.Latest.Dimensions.Geometry.Center()

	ws.ActiveLayout().SwapClient(active, target)
	tr.Tile(ws)
	store.ActiveWindowSet(store.X, active.Window)
	if movePointer {
		store.PointerMove(store.X, targetCenter)
	}

	return true
}

// moveDirectionAcrossScreens handles Super+Shift+Direction when there is no
// in-screen swap target. Instead of dumping the window in the center of the
// destination screen (the legacy behavior), find the nearest visible client on
// that screen and BSP-insert against its facing edge — mirroring how focus
// crosses the monitor boundary. With the destination known up front we can
// also warp the pointer synchronously instead of polling.
func moveDirectionAcrossScreens(tr *desktop.Tracker, source *desktop.Workspace, active *store.Client, d common.Direction, movePointer bool) bool {
	screen, ok := DirectionScreen(source.Location.Screen, d)
	if !ok {
		return false
	}
	target := tr.WorkspaceAt(source.Location.Desktop, screen)
	if target == nil || target == source {
		return false
	}

	// Pick the client on the destination screen closest to the source side.
	pivot := edgeClientForArrival(target, active, d)

	source.RemoveClient(active)
	active.Latest.Location.Screen = screen

	if pivot != nil {
		target.ActiveLayout().GetManager().InsertClient(active, pivot, arrivalEdge(d))
	} else {
		target.AddClient(active)
	}

	if source.TilingEnabled() {
		tr.Tile(source)
	}
	if target.TilingEnabled() {
		tr.Tile(target)
	} else {
		active.Restore(store.Latest)
	}

	store.ActiveWindowSet(store.X, active.Window)
	if movePointer {
		// Tile has updated the new geometry; warp directly there.
		store.PointerMove(store.X, active.Latest.Dimensions.Geometry.Center())
	}

	return true
}

// edgeClientForArrival returns the client on target that sits closest to the
// edge the moving window arrives from. Moving Right → leftmost client on the
// destination, etc.
func edgeClientForArrival(target *desktop.Workspace, moving *store.Client, d common.Direction) *store.Client {
	var best *store.Client
	var bestVal int
	for _, c := range target.VisibleClients() {
		if c == nil || c == moving {
			continue
		}
		x, y, w, h := c.Latest.Dimensions.Geometry.Pieces()
		var val int
		switch d {
		case common.Right:
			val = -x // smaller x wins
		case common.Left:
			val = x + w // larger x+w wins
		case common.Down:
			val = -y
		case common.Up:
			val = y + h
		}
		if best == nil || val > bestVal {
			best = c
			bestVal = val
		}
	}
	return best
}

// arrivalEdge is the BSP edge on the pivot where the moving window should land
// — the side facing the source screen.
func arrivalEdge(d common.Direction) string {
	switch d {
	case common.Right:
		return "left"
	case common.Left:
		return "right"
	case common.Down:
		return "top"
	case common.Up:
		return "bottom"
	}
	return ""
}

func DirectionScreen(source uint, d common.Direction) (uint, bool) {
	if int(source) >= len(store.Workplace.Displays.Screens) {
		return 0, false
	}

	sourceCenter := store.Workplace.Displays.Screens[source].Geometry.Center()
	target := uint(0)
	found := false
	minPrimary, minSecondary := 0, 0

	for i, screen := range store.Workplace.Displays.Screens {
		if uint(i) == source {
			continue
		}

		center := screen.Geometry.Center()
		dx, dy := center.X-sourceCenter.X, center.Y-sourceCenter.Y
		primary, secondary := 0, 0

		switch d {
		case common.Up:
			if dy >= 0 {
				continue
			}
			primary, secondary = common.AbsInt(dx), common.AbsInt(dy)
		case common.Down:
			if dy <= 0 {
				continue
			}
			primary, secondary = common.AbsInt(dx), common.AbsInt(dy)
		case common.Left:
			if dx >= 0 {
				continue
			}
			primary, secondary = common.AbsInt(dy), common.AbsInt(dx)
		case common.Right:
			if dx <= 0 {
				continue
			}
			primary, secondary = common.AbsInt(dy), common.AbsInt(dx)
		}

		if !found || primary < minPrimary || (primary == minPrimary && secondary < minSecondary) {
			target = uint(i)
			minPrimary = primary
			minSecondary = secondary
			found = true
		}
	}

	return target, found
}

func pointerInsideClient(c *store.Client) bool {
	return c != nil && store.Pointer != nil &&
		common.IsInsideRect(store.Pointer.Position, c.Latest.Dimensions.Geometry)
}

func shouldWarpPointer(c *store.Client) bool {
	return common.Config.WindowPointerWarp && pointerInsideClient(c)
}

func NextPosition(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	c1 := ws.ActiveLayout().ActiveClient()
	if c1 == nil {
		return false
	}
	c2 := ws.ActiveLayout().NextClient()
	if c2 == nil {
		return false
	}

	ws.ActiveLayout().SwapClient(c1, c2)
	tr.Tile(ws)
	store.ActiveWindowSet(store.X, c1.Window)

	return true
}

func PreviousPosition(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	c1 := ws.ActiveLayout().ActiveClient()
	if c1 == nil {
		return false
	}
	c2 := ws.ActiveLayout().PreviousClient()
	if c2 == nil {
		return false
	}

	ws.ActiveLayout().SwapClient(c1, c2)
	tr.Tile(ws)
	store.ActiveWindowSet(store.X, c1.Window)

	return true
}

func NextScreen(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	c := tr.ActiveClient()
	if c == nil {
		return false
	}
	screen := int(c.Latest.Location.Screen) + 1
	if screen > int(store.Workplace.ScreenCount)-1 {
		return false
	}

	return MoveWindowToScreen(tr, c, uint32(screen))
}

func PreviousScreen(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	c := tr.ActiveClient()
	if c == nil {
		return false
	}
	screen := int(c.Latest.Location.Screen) - 1
	if screen < 0 {
		return false
	}

	return MoveWindowToScreen(tr, c, uint32(screen))
}

func MoveWindowToScreen(tr *desktop.Tracker, c *store.Client, screen uint32) bool {
	source := tr.ClientWorkspace(c)
	if source == nil {
		return false
	}
	target := tr.WorkspaceAt(source.Location.Desktop, uint(screen))
	if target == nil || target == source {
		return false
	}

	movePointer := shouldWarpPointer(c)
	if !c.MoveToScreenDirect(screen) {
		return false
	}

	// Transfer layout ownership immediately instead of waiting for pointer events.
	source.RemoveClient(c)
	c.Latest.Location.Screen = uint(screen)
	target.AddClient(c)
	if source.TilingEnabled() {
		tr.Tile(source)
	}
	if target.TilingEnabled() {
		tr.Tile(target)
	} else {
		c.Restore(store.Latest)
	}

	store.ActiveWindowSet(store.X, c.Window)
	if movePointer {
		movePointerToClientScreen(c, uint(screen))
	}

	return true
}

func movePointerToClientScreen(c *store.Client, screen uint) {
	go func() {
		var previous common.Point
		stable := 0

		for range 20 {
			time.Sleep(50 * time.Millisecond)
			if c.Latest.Location.Screen != screen {
				return
			}

			rect, err := c.Window.Instance.DecorGeometry()
			if err != nil {
				continue
			}
			center := common.CreateGeometry(rect).Center()
			if store.ScreenGet(center) != screen {
				stable = 0
				continue
			}

			if center == previous {
				stable++
			} else {
				previous = center
				stable = 0
			}
			if stable >= 2 {
				store.PointerMove(store.X, center)
				return
			}
		}
	}()
}

func IncreaseProportion(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.ActiveLayout().IncreaseProportion()
	tr.Tile(ws)

	return true
}

func DecreaseProportion(tr *desktop.Tracker, ws *desktop.Workspace) bool {
	if ws.TilingDisabled() {
		return false
	}
	ws.ActiveLayout().DecreaseProportion()
	tr.Tile(ws)

	return true
}

func DirectionProportion(tr *desktop.Tracker, ws *desktop.Workspace, direction common.Direction) bool {
	if ws.TilingDisabled() {
		return false
	}
	if !ws.ActiveLayout().GetManager().DirectionProportion(direction) {
		return false
	}
	tr.Tile(ws)

	return true
}

func ReloadConfig(tr *desktop.Tracker) bool {
	if !common.ReloadConfig() {
		return false
	}

	ReloadKeys(tr)
	ReloadMouseBindings(tr)

	for _, ws := range tr.Workspaces {
		for _, layout := range ws.Layouts {
			layout.GetManager().Decoration = common.Config.WindowDecoration
		}
		if ws.TilingEnabled() {
			tr.Tile(ws)
		}
	}

	log.Info("Reload config")
	return true
}

func Restart(tr *desktop.Tracker) bool {
	xevent.Detach(store.X, store.X.RootWin())

	for _, ws := range tr.Workspaces {
		if ws.TilingDisabled() {
			continue
		}
		ws.DisableTiling()
		tr.Restore(ws, store.Latest)
	}

	log.Info("Restart")

	// Communicate application exit
	Disconnect()

	// Restart application
	syscall.Exec(common.Process.Path, os.Args, os.Environ())

	return true
}

func Exit(tr *desktop.Tracker) bool {
	xevent.Detach(store.X, store.X.RootWin())

	for _, ws := range tr.Workspaces {
		if ws.TilingDisabled() {
			continue
		}
		ws.DisableTiling()
		tr.Restore(ws, store.Latest)
	}

	log.Info("Exit")

	// Communicate application exit
	Disconnect()

	// Exit application
	os.Exit(0)

	return true
}

func External(command string) bool {
	params := strings.Split(command, " ")

	if !common.HasFlag("enable-external-commands") {
		log.Warn("Executing external command \"", params[0], "\" disabled")
		return false
	}

	log.Info("Executing external command \"", params[0], " ", params[1:], "\"")

	// Execute external command
	cmd := exec.Command(params[0], params[1:]...)
	if err := cmd.Run(); err != nil {
		log.Error("External command failed: ", err)
		return false
	}

	return true
}

func OnExecute(fun func(string, uint, uint)) {
	executeCallbacksFun = append(executeCallbacksFun, fun)
}

func executeCallbacks(action string, desktop uint, screen uint) {
	log.Info("Execute event ", action)

	for _, fun := range executeCallbacksFun {
		fun(action, desktop, screen)
	}
}
