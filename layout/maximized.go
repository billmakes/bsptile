package layout

import (
	"math"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

type MaximizedLayout struct {
	Name           string // Layout name
	*store.Manager        // Layout store manager
}

func CreateMaximizedLayout(manager *store.Manager) *MaximizedLayout {
	layout := &MaximizedLayout{
		Name:    "maximized",
		Manager: manager,
	}
	layout.Reset()
	return layout
}

func (l *MaximizedLayout) Reset() {
	l.Manager.Reset()
}

func (l *MaximizedLayout) Apply() {
	active := l.ActiveClient()
	if active == nil {
		return
	}

	dx, dy, dw, dh := store.DesktopGeometry(l.Location.Screen).Pieces()
	gap := common.Config.WindowGapSize

	log.Info("Tile active window with ", l.Name, " layout [workspace-", l.Location.Desktop, "-", l.Location.Screen, "]")

	for _, c := range l.Clients(store.Ordered) {
		if c != active {
			c.UnLimit()
			c.UnFullscreen()
		}
	}

	minw := int(math.Round(float64(dw - 2*gap)))
	minh := int(math.Round(float64(dh - 2*gap)))
	active.Limit(minw, minh)
	active.MoveWindow(dx+gap, dy+gap, dw-2*gap, dh-2*gap)
}

func (l *MaximizedLayout) UpdateProportions(c *store.Client, d *store.Directions, geom common.Geometry) {
	l.Reset()
}

func (l *MaximizedLayout) GetManager() *store.Manager {
	return l.Manager
}

func (l *MaximizedLayout) GetName() string {
	return l.Name
}
