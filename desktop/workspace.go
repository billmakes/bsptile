package desktop

import (
	"fmt"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/layout"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

type Workspace struct {
	Name     string         // Workspace location name
	Location store.Location // Desktop and screen location
	Layouts  []Layout       // List of available layouts
	Layout   uint           // Active layout index
	Tiling   bool           // Tiling is enabled
}

func CreateWorkspaces() map[store.Location]*Workspace {
	workspaces := make(map[store.Location]*Workspace)

	for desktop := uint(0); desktop < store.Workplace.DesktopCount; desktop++ {
		for screen := uint(0); screen < store.Workplace.ScreenCount; screen++ {
			location := store.Location{Desktop: desktop, Screen: screen}

			// Create layouts for each desktop and screen
			ws := &Workspace{
				Name:     fmt.Sprintf("workspace-%d-%d", location.Desktop, location.Screen),
				Location: location,
				Layouts:  CreateLayouts(location),
			}

			ws.ApplyConfig()

			// Map location to workspace
			workspaces[location] = ws
		}
	}

	return workspaces
}

func (ws *Workspace) ApplyConfig() {
	ws.Tiling = common.Config.TilingEnabled
	ws.Layout = 0
	for _, layout := range ws.Layouts {
		layout.GetManager().Decoration = common.Config.WindowDecoration
	}
	applyWorkspaceRule(ws, common.MatchWorkspaceRule(ws.Location.Desktop, ws.Location.Screen))
}

func applyWorkspaceRule(ws *Workspace, rule *common.WorkspaceRule) {
	if rule == nil {
		return
	}
	if rule.Tiling != nil {
		ws.Tiling = *rule.Tiling
	}
	if rule.Layout != "" {
		for i, l := range ws.Layouts {
			if l.GetName() == rule.Layout {
				ws.Layout = uint(i)
				break
			}
		}
	}
	if rule.Decoration != nil {
		for _, l := range ws.Layouts {
			l.GetManager().Decoration = *rule.Decoration
		}
	}
}

func CreateLayouts(loc store.Location) []Layout {
	manager := store.CreateBSPManager(loc)
	return []Layout{
		layout.CreateBSPLayout(manager),
		layout.CreateMaximizedLayout(manager),
		layout.CreateFullscreenLayout(manager),
	}
}

func (ws *Workspace) EnableTiling() {
	ws.Tiling = true
}

func (ws *Workspace) DisableTiling() {
	ws.Tiling = false
}

func (ws *Workspace) TilingEnabled() bool {
	if ws == nil {
		return false
	}
	return ws.Tiling
}

func (ws *Workspace) TilingDisabled() bool {
	if ws == nil {
		return true
	}
	return !ws.Tiling
}

func (ws *Workspace) ActiveLayout() Layout {
	if int(ws.Layout) >= len(ws.Layouts) {
		ws.SetLayout(0)
	}
	return ws.Layouts[ws.Layout]
}

func (ws *Workspace) ActiveWindowLayout() bool {
	if ws == nil {
		return false
	}
	return common.IsInList(ws.ActiveLayout().GetName(), []string{"maximized", "fullscreen"})
}

func (ws *Workspace) SetLayout(layout uint) {
	if int(layout) >= len(ws.Layouts) {
		layout = 0
	}
	ws.Layout = layout
}

func (ws *Workspace) ResetLayouts() {

	// Reset layouts
	for _, l := range ws.Layouts {

		// Reset client decorations
		mg := l.GetManager()
		mg.Decoration = common.Config.WindowDecoration

		// Reset layout proportions
		l.Reset()
	}
}

func (ws *Workspace) AddClient(c *store.Client) {
	log.Info("Add client to workspace tree [", c.Latest.Class, "]")
	ws.Layouts[0].AddClient(c)
}

func (ws *Workspace) RemoveClient(c *store.Client) {
	log.Info("Remove client from workspace tree [", c.Latest.Class, "]")
	ws.Layouts[0].RemoveClient(c)
}

func (ws *Workspace) VisibleClients() []*store.Client {
	al := ws.ActiveLayout()
	mg := al.GetManager()

	// Obtain visible clients
	clients := mg.Clients(store.Stacked)
	if ws.ActiveWindowLayout() {
		active := mg.ActiveClient()
		if active != nil {
			return []*store.Client{active}
		}
		ordered := mg.Clients(store.Ordered)
		if len(ordered) > 0 {
			return []*store.Client{ordered[len(ordered)-1]}
		}
	}

	return clients
}

func (ws *Workspace) Tile() {
	if ws.TilingDisabled() {
		return
	}
	mg := ws.ActiveLayout().GetManager()
	clients := mg.Clients(store.Stacked)

	// Set client decorations
	for _, c := range clients {
		if c == nil {
			continue
		}
		if mg.DecorationEnabled() {
			if c.Decorate() {
				c.Update()
			}
		} else {
			if c.UnDecorate() {
				c.Update()
			}
		}
	}

	// Apply active layout
	ws.ActiveLayout().Apply()
}

func (ws *Workspace) Restore(flag uint8) {
	mg := ws.ActiveLayout().GetManager()
	clients := mg.Clients(store.Stacked)

	log.Info("Untile ", len(clients), " windows [", ws.Name, "]")

	// Restore client dimensions
	for _, c := range clients {
		if c == nil {
			continue
		}
		c.Restore(flag)
	}
}
