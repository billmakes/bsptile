package common

import "testing"

func setConfigRules(t *testing.T, window []WindowRule, workspace []WorkspaceRule) {
	t.Helper()
	previousW, previousWS := Config.WindowRules, Config.WorkspaceRules
	t.Cleanup(func() {
		Config.WindowRules = previousW
		Config.WorkspaceRules = previousWS
	})
	Config.WindowRules = window
	Config.WorkspaceRules = workspace
}

func ptrInt(v int) *int    { return &v }
func ptrBool(v bool) *bool { return &v }

func TestMatchWindowRuleFirstMatchWins(t *testing.T) {
	setConfigRules(t, []WindowRule{
		{Class: "^pavucontrol$", Floating: true},
		{Class: "(?i)firefox", Monitor: ptrInt(2)},
		{Class: "(?i)firefox", Monitor: ptrInt(3)}, // shadowed by the earlier match
	}, nil)

	r := MatchWindowRule("Firefox", "Mozilla Firefox")
	if r == nil {
		t.Fatal("expected a match")
	}
	if r.Monitor == nil || *r.Monitor != 2 {
		t.Fatalf("monitor = %v, want 2 (first match wins)", r.Monitor)
	}
}

func TestMatchWindowRuleNameNarrowsMatch(t *testing.T) {
	setConfigRules(t, []WindowRule{
		{Class: "Steam", Name: "^Friends List$", Floating: true},
		{Class: "Calculator", Sticky: true},
		{Class: "Steam", Tile: true},
	}, nil)

	if r := MatchWindowRule("Steam", "Friends List"); r == nil || !r.Floating {
		t.Fatalf("name-scoped rule did not match: %+v", r)
	}
	if r := MatchWindowRule("Steam", "Library"); r == nil || !r.Tile {
		t.Fatalf("class-only fallback did not match: %+v", r)
	}
	if r := MatchWindowRule("Calculator", "Calculator"); r == nil || !r.Sticky {
		t.Fatalf("sticky rule did not match: %+v", r)
	}
}

func TestMatchWindowRuleNoMatch(t *testing.T) {
	setConfigRules(t, []WindowRule{
		{Class: "nope"},
	}, nil)
	if r := MatchWindowRule("Other", ""); r != nil {
		t.Fatalf("unexpected match: %+v", r)
	}
}

func TestMatchWindowRuleSkipsEmptyClass(t *testing.T) {
	setConfigRules(t, []WindowRule{
		{Class: ""}, // ill-formed, must be ignored
		{Class: "(?i)firefox", Monitor: ptrInt(2)},
	}, nil)
	r := MatchWindowRule("Firefox", "")
	if r == nil || r.Monitor == nil || *r.Monitor != 2 {
		t.Fatalf("empty-class rule should not block later matches: %+v", r)
	}
}

func TestMatchWorkspaceRuleDesktopOnly(t *testing.T) {
	setConfigRules(t, nil, []WorkspaceRule{
		{Desktop: 2, Tiling: ptrBool(false)},
	})
	// 0-indexed desktop=1, any screen
	r := MatchWorkspaceRule(1, 0)
	if r == nil || r.Tiling == nil || *r.Tiling != false {
		t.Fatalf("desktop-only rule did not match: %+v", r)
	}
	r = MatchWorkspaceRule(1, 5) // also any other screen
	if r == nil {
		t.Fatal("desktop-only rule should ignore screen index")
	}
}

func TestMatchWorkspaceRuleScreenScoped(t *testing.T) {
	setConfigRules(t, nil, []WorkspaceRule{
		{Desktop: 3, Screen: ptrInt(1), Layout: "maximized"},
		{Desktop: 3, Layout: "bsp"},
	})

	// 0-indexed desktop=2, screen=0 → matches the screen-scoped rule first.
	if r := MatchWorkspaceRule(2, 0); r == nil || r.Layout != "maximized" {
		t.Fatalf("screen-scoped rule should win on screen 1: %+v", r)
	}
	// 0-indexed desktop=2, screen=1 → screen-scoped rule does not match, falls through.
	if r := MatchWorkspaceRule(2, 1); r == nil || r.Layout != "bsp" {
		t.Fatalf("fallback rule did not match: %+v", r)
	}
}

func TestMatchWorkspaceRuleMiss(t *testing.T) {
	setConfigRules(t, nil, []WorkspaceRule{
		{Desktop: 1, Tiling: ptrBool(false)},
	})
	if r := MatchWorkspaceRule(2, 0); r != nil {
		t.Fatalf("unexpected match: %+v", r)
	}
}
