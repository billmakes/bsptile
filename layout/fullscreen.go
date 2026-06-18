package layout

import (
	"math"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

type FullscreenLayout struct {
	Name           string // Layout name
	*store.Manager        // Layout store manager
}

func CreateFullscreenLayout(manager *store.Manager) *FullscreenLayout {
	layout := &FullscreenLayout{
		Name:    "fullscreen",
		Manager: manager,
	}
	layout.Reset()
	return layout
}

func (l *FullscreenLayout) Reset() {
	l.Manager.Reset()
}

func (l *FullscreenLayout) Apply() {
	clients := l.Clients(store.Ordered)

	_, _, dw, dh := store.ScreenGeometry(l.Location.Screen).Pieces()

	csize := len(clients)

	log.Info("Tile ", csize, " windows with ", l.Name, " layout [workspace-", l.Location.Desktop, "-", l.Location.Screen, "]")

	// Main area layout
	for _, c := range clients {

		// Limit minimum dimensions
		minw := int(math.Round(float64(dw)))
		minh := int(math.Round(float64(dh)))
		c.Limit(minw, minh)

		// Make window fullscreen
		c.Fullscreen()
		c.Update()
	}
}

func (l *FullscreenLayout) UpdateProportions(c *store.Client, d *store.Directions, geom common.Geometry) {
	l.Reset()
}

func (l *FullscreenLayout) GetManager() *store.Manager {
	return l.Manager
}

func (l *FullscreenLayout) GetName() string {
	return l.Name
}
