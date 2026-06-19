package input

import (
	"sort"
	"sync"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
)

type ActionSpec struct {
	Name        string
	Description string
	Handler     func(*desktop.Tracker, *desktop.Workspace) bool
}

type ActionInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var (
	actionRegistry     map[string]ActionSpec
	actionRegistryOnce sync.Once
)

func buildActionRegistry() map[string]ActionSpec {
	specs := []ActionSpec{
		{Name: "enable", Description: "enable tiling", Handler: EnableTiling},
		{Name: "disable", Description: "disable tiling", Handler: DisableTiling},
		{Name: "toggle", Description: "toggle tiling", Handler: ToggleTiling},
		{Name: "decoration", Description: "toggle window decorations", Handler: ToggleDecoration},
		{Name: "restore", Description: "restore window geometry", Handler: Restore},
		{Name: "reset", Description: "reset BSP split ratios", Handler: Reset},
		{Name: "close", Description: "close active window", Handler: CloseWindow},
		{Name: "balance", Description: "balance the BSP tree", Handler: Balance},
		{Name: "tree_rotate", Description: "rotate the BSP tree", Handler: RotateTree},
		{Name: "layout_bsp", Description: "activate BSP layout", Handler: BSPLayout},
		{Name: "layout_maximized", Description: "toggle maximized layout", Handler: MaximizedLayout},
		{Name: "layout_fullscreen", Description: "toggle fullscreen layout", Handler: FullscreenLayout},
		{Name: "window_next", Description: "focus next window", Handler: NextWindow},
		{Name: "window_previous", Description: "focus previous window", Handler: PreviousWindow},
		{Name: "window_up", Description: "focus window above", Handler: directionAction(common.Up)},
		{Name: "window_down", Description: "focus window below", Handler: directionAction(common.Down)},
		{Name: "window_left", Description: "focus window left", Handler: directionAction(common.Left)},
		{Name: "window_right", Description: "focus window right", Handler: directionAction(common.Right)},
		{Name: "move_window_up", Description: "move window up", Handler: moveDirectionAction(common.Up)},
		{Name: "move_window_down", Description: "move window down", Handler: moveDirectionAction(common.Down)},
		{Name: "move_window_left", Description: "move window left", Handler: moveDirectionAction(common.Left)},
		{Name: "move_window_right", Description: "move window right", Handler: moveDirectionAction(common.Right)},
		{Name: "position_next", Description: "move window to next BSP position", Handler: NextPosition},
		{Name: "position_previous", Description: "move window to previous BSP position", Handler: PreviousPosition},
		{Name: "screen_next", Description: "move window to next screen", Handler: NextScreen},
		{Name: "screen_previous", Description: "move window to previous screen", Handler: PreviousScreen},
		{Name: "window_to_desktop_next", Description: "move window to next desktop", Handler: NextDesktop},
		{Name: "window_to_desktop_previous", Description: "move window to previous desktop", Handler: PreviousDesktop},
		{Name: "desktop_next", Description: "switch to next desktop", Handler: NextDesktopView},
		{Name: "desktop_previous", Description: "switch to previous desktop", Handler: PreviousDesktopView},
		{Name: "proportion_increase", Description: "increase active split ratio", Handler: IncreaseProportion},
		{Name: "proportion_decrease", Description: "decrease active split ratio", Handler: DecreaseProportion},
		{Name: "resize_left", Description: "resize active window left", Handler: resizeAction(common.Left)},
		{Name: "resize_right", Description: "resize active window right", Handler: resizeAction(common.Right)},
		{Name: "resize_up", Description: "resize active window up", Handler: resizeAction(common.Up)},
		{Name: "resize_down", Description: "resize active window down", Handler: resizeAction(common.Down)},
		{Name: "proportion_up", Description: "move horizontal split up", Handler: proportionAction(common.Up)},
		{Name: "proportion_down", Description: "move horizontal split down", Handler: proportionAction(common.Down)},
		{Name: "proportion_left", Description: "move vertical split left", Handler: proportionAction(common.Left)},
		{Name: "proportion_right", Description: "move vertical split right", Handler: proportionAction(common.Right)},
		{Name: "reload", Description: "reload configuration", Handler: trackerAction(ReloadConfig)},
		{Name: "restart", Description: "restart bsptile", Handler: trackerAction(Restart)},
		{Name: "exit", Description: "exit bsptile", Handler: trackerAction(Exit)},
	}

	registry := make(map[string]ActionSpec, len(specs))
	for _, spec := range specs {
		registry[spec.Name] = spec
	}
	return registry
}

func LookupAction(name string) (ActionSpec, bool) {
	spec, ok := actions()[name]
	return spec, ok
}

func ActionNames() []string {
	registry := actions()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ActionInfos() []ActionInfo {
	registry := actions()
	infos := make([]ActionInfo, 0, len(registry)+1)
	for _, spec := range registry {
		infos = append(infos, ActionInfo{
			Name:        spec.Name,
			Description: spec.Description,
		})
	}
	infos = append(infos, ActionInfo{
		Name:        "window_to_desktop_<n>",
		Description: "move window to numbered desktop",
	})
	infos = append(infos, ActionInfo{
		Name:        "desktop_<n>",
		Description: "switch to numbered desktop",
	})
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

func actions() map[string]ActionSpec {
	actionRegistryOnce.Do(func() {
		actionRegistry = buildActionRegistry()
	})
	return actionRegistry
}

func trackerAction(handler func(*desktop.Tracker) bool) func(*desktop.Tracker, *desktop.Workspace) bool {
	return func(tr *desktop.Tracker, _ *desktop.Workspace) bool {
		return handler(tr)
	}
}
func directionAction(direction common.Direction) func(*desktop.Tracker, *desktop.Workspace) bool {
	return func(tr *desktop.Tracker, ws *desktop.Workspace) bool {
		return DirectionWindow(tr, ws, direction)
	}
}

func moveDirectionAction(direction common.Direction) func(*desktop.Tracker, *desktop.Workspace) bool {
	return func(tr *desktop.Tracker, ws *desktop.Workspace) bool {
		return MoveDirectionWindow(tr, ws, direction)
	}
}

func resizeAction(direction common.Direction) func(*desktop.Tracker, *desktop.Workspace) bool {
	return func(tr *desktop.Tracker, ws *desktop.Workspace) bool {
		return Resize(tr, ws, direction)
	}
}

func proportionAction(direction common.Direction) func(*desktop.Tracker, *desktop.Workspace) bool {
	return func(tr *desktop.Tracker, ws *desktop.Workspace) bool {
		return DirectionProportion(tr, ws, direction)
	}
}
