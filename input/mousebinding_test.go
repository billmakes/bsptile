package input

import "testing"

func TestNormalizeMouseBinding(t *testing.T) {
	tests := map[string]string{
		"8":                    "8",
		"Button8":              "8",
		"button12":             "12",
		"Mod4-Button12":        "Mod4-12",
		"Button1-Mod4-Button8": "Button1-Mod4-8",
		"Button0":              "Button0",
		"Button256":            "Button256",
	}

	for binding, want := range tests {
		if got := normalizeMouseBinding(binding); got != want {
			t.Fatalf("normalizeMouseBinding(%q) = %q, want %q", binding, got, want)
		}
	}
}
