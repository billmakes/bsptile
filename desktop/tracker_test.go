package desktop

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/billmakes/bsptile/v2/store"
)

func TestClientForWindowUsesRequestedWindowInsteadOfStaleActiveCache(t *testing.T) {
	stale := &store.Client{Window: &store.XWindow{Id: 1}}
	focused := &store.Client{Window: &store.XWindow{Id: 2}}
	tracker := &Tracker{
		Clients: map[xproto.Window]*store.Client{
			stale.Window.Id:   stale,
			focused.Window.Id: focused,
		},
	}

	if client := tracker.ClientForWindow(*focused.Window); client != focused {
		t.Fatalf("resolved client = %v, want focused client", client)
	}
}
