package layout

import (
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

type BSPLayout struct {
	Name string
	*store.Manager
}

func CreateBSPLayout(manager *store.Manager) *BSPLayout {
	return &BSPLayout{
		Name:    "bsp",
		Manager: manager,
	}
}

func (l *BSPLayout) Reset() {
	l.Manager.Reset()
}

func (l *BSPLayout) Apply() {
	for _, c := range l.Clients(store.Stacked) {
		c.UnFullscreen()
	}
	geom := *store.DesktopGeometry(l.Location.Screen)
	log.Info("Tile ", len(l.Clients(store.Stacked)), " windows with BSP layout [workspace-",
		l.Location.Desktop, "-", l.Location.Screen, "]")
	l.Manager.Apply(geom)
}

func (l *BSPLayout) GetManager() *store.Manager { return l.Manager }
func (l *BSPLayout) GetName() string            { return l.Name }
