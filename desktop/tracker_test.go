package desktop

import (
	"sync"
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/billmakes/bsptile/v2/layout"
	"github.com/billmakes/bsptile/v2/store"
)

func newTaskTracker() *Tracker {
	tracker := &Tracker{tasks: make(chan trackerTask, 64)}
	go tracker.runTasks()
	return tracker
}

func TestTrackerCallWaitsForEarlierPostedWork(t *testing.T) {
	tracker := newTaskTracker()
	var mu sync.Mutex
	order := []int{}

	tracker.Post(func() {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	tracker.Call(func() {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("execution order = %v, want [1 2]", order)
	}
}

func TestTrackerWithoutTaskLaneExecutesInline(t *testing.T) {
	tracker := &Tracker{}
	called := false
	tracker.Call(func() {
		called = true
	})
	if !called {
		t.Fatal("inline tracker call did not execute")
	}
}

func TestTrackerTaskPanicDoesNotStopLane(t *testing.T) {
	tracker := newTaskTracker()
	tracker.Call(func() {
		panic("test")
	})

	called := false
	tracker.Call(func() {
		called = true
	})
	if !called {
		t.Fatal("tracker lane stopped after a task panic")
	}
}

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
