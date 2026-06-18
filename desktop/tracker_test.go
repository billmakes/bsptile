package desktop

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/billmakes/bsptile/v2/layout"
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

func TestActiveWindowLayout(t *testing.T) {
	manager := store.CreateBSPManager(store.Location{})
	tests := []struct {
		layouts []Layout
		active  uint
		want    bool
	}{
		{layouts: []Layout{layout.CreateBSPLayout(manager)}, want: false},
		{layouts: []Layout{layout.CreateMaximizedLayout(manager)}, want: true},
		{layouts: []Layout{layout.CreateFullscreenLayout(manager)}, want: true},
	}

	for _, test := range tests {
		ws := &Workspace{Layouts: test.layouts, Layout: test.active}
		if got := ws.ActiveWindowLayout(); got != test.want {
			t.Fatalf("layout %q active-window status = %v, want %v", ws.ActiveLayout().GetName(), got, test.want)
		}
	}
}
