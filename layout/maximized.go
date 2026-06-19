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

	target := MaximizedGeometry(outerGapGeometry(*store.DesktopGeometry(l.Location.Screen)), common.Config.WindowGapSize)

	log.Info("Tile active window with ", l.Name, " layout [workspace-", l.Location.Desktop, "-", l.Location.Screen, "]")

	for _, c := range l.Clients(store.Ordered) {
		if c != active {
			c.UnLimit()
			c.UnFullscreen()
		}
	}

	minw := int(math.Round(float64(target.Width)))
	minh := int(math.Round(float64(target.Height)))
	active.Limit(minw, minh)
	active.MoveWindow(target.X, target.Y, target.Width, target.Height)
}

func MaximizedGeometry(desktop common.Geometry, gap int) common.Geometry {
	width := common.MaxInt(desktop.Width-2*gap, 1)
	height := common.MaxInt(desktop.Height-2*gap, 1)
	return common.Geometry{
		X:      desktop.X + gap,
		Y:      desktop.Y + gap,
		Width:  width,
		Height: height,
	}
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
