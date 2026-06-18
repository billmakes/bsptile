package input

import "testing"

func TestKeyModeAction(t *testing.T) {
	tests := []struct {
		action string
		mode   string
		ok     bool
	}{
		{action: "mode_default", mode: "default", ok: true},
		{action: "mode_resize", mode: "resize", ok: true},
		{action: "mode_", ok: false},
		{action: "window_left", ok: false},
	}

	for _, test := range tests {
		mode, ok := keyModeAction(test.action)
		if mode != test.mode || ok != test.ok {
			t.Fatalf("keyModeAction(%q) = %q, %v; want %q, %v", test.action, mode, ok, test.mode, test.ok)
		}
	}
}
