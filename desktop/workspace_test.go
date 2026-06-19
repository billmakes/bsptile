package desktop

import (
	"testing"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/store"
)

func TestWorkspaceApplyConfigReappliesDefaultsAndRule(t *testing.T) {
	previous := common.Config
	t.Cleanup(func() {
		common.Config = previous
	})

	tiling := false
	decoration := false
	common.Config.TilingEnabled = true
	common.Config.WindowDecoration = true
	common.Config.WorkspaceRules = []common.WorkspaceRule{{
		Desktop:    1,
		Screen:     ptrIntWorkspace(1),
		Tiling:     &tiling,
		Layout:     "fullscreen",
		Decoration: &decoration,
	}}

	ws := &Workspace{
		Location: store.Location{},
		Layouts:  CreateLayouts(store.Location{}),
	}
	ws.ApplyConfig()

	if ws.TilingEnabled() {
		t.Fatal("workspace rule did not disable tiling")
	}
	if got := ws.ActiveLayout().GetName(); got != "fullscreen" {
		t.Fatalf("layout = %q, want fullscreen", got)
	}
	for _, layout := range ws.Layouts {
		if layout.GetManager().DecorationEnabled() {
			t.Fatal("workspace rule did not disable decoration")
		}
	}
}

func ptrIntWorkspace(value int) *int {
	return &value
}
