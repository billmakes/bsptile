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
	for _, name := range []string{
		"layout_bsp",
		"layout_maximized",
		"layout_fullscreen",
		"close",
		"desktop_next",
		"desktop_previous",
		"reload",
		"restart",
		"exit",
	} {
		if _, ok := LookupAction(name); !ok {
			t.Fatalf("missing action %q", name)
		}
	}
}

func TestActionInfosIncludesDynamicNumberedPatterns(t *testing.T) {
	found := map[string]bool{}
	for _, info := range ActionInfos() {
		if info.Description == "" {
			t.Fatalf("action info %q is missing description", info.Name)
		}
		found[info.Name] = true
	}
	for _, name := range []string{"desktop_<n>", "window_to_desktop_<n>"} {
		if !found[name] {
			t.Fatalf("missing dynamic action pattern %q", name)
		}
	}
}
