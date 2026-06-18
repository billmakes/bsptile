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
	if path := os.Getenv("CORTILE_TEST_CONFIG"); path != "" {
		paths = append(paths, path)
	}

	for _, path := range paths {
		var config Configuration
		if _, err := toml.DecodeFile(path, &config); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
}
