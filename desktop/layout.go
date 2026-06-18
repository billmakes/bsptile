package desktop

import (
	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"
)

type Layout interface {
	Reset()
	Apply()
	AddClient(c *store.Client)
	RemoveClient(c *store.Client)
	SwapClient(c1 *store.Client, c2 *store.Client)
	ActiveClient() *store.Client
	NextClient() *store.Client
	PreviousClient() *store.Client
	DirectionClient(d common.Direction) *store.Client
	Rotate()
	IncreaseProportion()
	DecreaseProportion()
	UpdateProportions(c *store.Client, d *store.Directions, geom common.Geometry)
	GetManager() *store.Manager
	GetName() string
}
