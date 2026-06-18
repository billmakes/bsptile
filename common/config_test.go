package common

import (
	"os"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestKeyBindingsDecodeStringAndArray(t *testing.T) {
	var config struct {
		Keys map[string]KeyBindings `toml:"keys"`
	}

	_, err := toml.Decode(`
[keys]
single = "Control-A"
multiple = ["Control-B", "Mod4-B"]
empty = ""
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
