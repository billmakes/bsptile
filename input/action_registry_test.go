package input

import "testing"

func TestActionRegistryHasUniqueUsableSpecs(t *testing.T) {
	names := ActionNames()
	if len(names) != len(actions()) {
		t.Fatalf("action names = %d, registry = %d", len(names), len(actions()))
	}
	for _, name := range names {
		spec, ok := LookupAction(name)
		if !ok {
			t.Fatalf("registered action %q cannot be looked up", name)
		}
		if spec.Name != name || spec.Description == "" || spec.Handler == nil {
			t.Fatalf("incomplete action spec: %+v", spec)
		}
	}
}

func TestActionRegistryIncludesLifecycleAndLayoutActions(t *testing.T) {
	for _, name := range []string{"layout_bsp", "layout_maximized", "layout_fullscreen", "reload", "restart", "exit"} {
		if _, ok := LookupAction(name); !ok {
			t.Fatalf("missing action %q", name)
		}
	}
}
