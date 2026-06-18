package common

import (
	"os"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestKeyBindingsDecodeStringAndArray(t *testing.T) {
	var config struct {
		Keys  map[string]KeyBindings `toml:"keys"`
		Mouse map[string]KeyBindings `toml:"mouse"`
		Modes map[string]Mode        `toml:"modes"`
	}

	_, err := toml.Decode(`
[keys]
single = "Control-A"
multiple = ["Control-B", "Mod4-B"]
empty = ""

[mouse]
window_next = "Button8"
window_previous = ["Button9", "Mod4-Button9"]

[modes.resize]
mode_default = ["Escape", "Return"]
increase = "Right"
`, &config)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]KeyBindings{
		"single":   {"Control-A"},
		"multiple": {"Control-B", "Mod4-B"},
		"empty":    {""},
	}
	if !reflect.DeepEqual(config.Keys, expected) {
		t.Fatalf("unexpected bindings: %#v", config.Keys)
	}

	expectedMouse := map[string]KeyBindings{
		"window_next":     {"Button8"},
		"window_previous": {"Button9", "Mod4-Button9"},
	}
	if !reflect.DeepEqual(config.Mouse, expectedMouse) {
		t.Fatalf("unexpected mouse bindings: %#v", config.Mouse)
	}

	expectedMode := Mode{
		"mode_default": {"Escape", "Return"},
		"increase":     {"Right"},
	}
	if !reflect.DeepEqual(config.Modes["resize"], expectedMode) {
		t.Fatalf("unexpected mode bindings: %#v", config.Modes["resize"])
	}
}

func TestConfigFilesDecode(t *testing.T) {
	paths := []string{"../config.toml"}
	if path := os.Getenv("BSPTILE_TEST_CONFIG"); path != "" {
		paths = append(paths, path)
	}

	for _, path := range paths {
		var config Configuration
		if _, err := toml.DecodeFile(path, &config); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
}

func TestReadConfigReplacesConfigOnlyAfterSuccessfulDecode(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	previous := Config
	t.Cleanup(func() {
		Config = previous
	})

	if err := os.WriteFile(path, []byte("[keys]\ntoggle = \"Control-T\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !readConfig(path, false) {
		t.Fatal("expected valid config to load")
	}
	if _, ok := Config.Keys["toggle"]; !ok {
		t.Fatal("expected toggle binding")
	}

	if err := os.WriteFile(path, []byte("[keys]\ninvalid = [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if readConfig(path, false) {
		t.Fatal("expected invalid config to be rejected")
	}
	if _, ok := Config.Keys["toggle"]; !ok {
		t.Fatal("invalid reload changed the running config")
	}

	if err := os.WriteFile(path, []byte("[keys]\nreload = \"Control-C\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !readConfig(path, false) {
		t.Fatal("expected replacement config to load")
	}
	if _, ok := Config.Keys["toggle"]; ok {
		t.Fatal("removed binding survived config replacement")
	}
}

func TestValidateConfigRequiresModeDefault(t *testing.T) {
	config := Configuration{
		Modes: map[string]Mode{
			"resize": {
				"proportion_increase": {"Right"},
			},
		},
	}

	if err := validateConfig(config); err == nil {
		t.Fatal("expected mode without mode_default to be rejected")
	}

	config.Modes["resize"]["mode_default"] = KeyBindings{"Escape"}
	if err := validateConfig(config); err != nil {
		t.Fatalf("expected valid mode: %v", err)
	}
}
