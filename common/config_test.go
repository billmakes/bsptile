package common

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"
)

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

	if err := os.WriteFile(path, []byte("[systray]\nclick_middle = \"toggle\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !readConfig(path, false) {
		t.Fatal("expected valid config to load")
	}
	if Config.Systray["click_middle"] != "toggle" {
		t.Fatal("expected systray action")
	}

	if err := os.WriteFile(path, []byte("[systray]\ninvalid = [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if readConfig(path, false) {
		t.Fatal("expected invalid config to be rejected")
	}
	if Config.Systray["click_middle"] != "toggle" {
		t.Fatal("invalid reload changed the running config")
	}

	if err := os.WriteFile(path, []byte("[systray]\nclick_middle = \"reload\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !readConfig(path, false) {
		t.Fatal("expected replacement config to load")
	}
	if Config.Systray["click_middle"] != "reload" {
		t.Fatal("replacement config was not applied")
	}
}

func TestValidateConfigRejectsMalformedWindowIgnore(t *testing.T) {
	tests := []Configuration{
		{WindowIgnore: [][]string{{"only-class"}}},
		{WindowIgnore: [][]string{{"[", ""}}},
		{WindowIgnore: [][]string{{"valid", "["}}},
	}
	for _, config := range tests {
		if err := validateConfig(&config); err == nil {
			t.Fatalf("expected invalid window_ignore to be rejected: %#v", config.WindowIgnore)
		}
	}
}

func TestValidateConfigRejectsInvalidRules(t *testing.T) {
	zero := 0
	one := 1
	tests := []Configuration{
		{WindowRules: []WindowRule{{}}},
		{WindowRules: []WindowRule{{Class: "["}}},
		{WindowRules: []WindowRule{{Class: "x", Name: "["}}},
		{WindowRules: []WindowRule{{Class: "x", Tile: true, Floating: true}}},
		{WindowRules: []WindowRule{{Class: "x", Monitor: &zero}}},
		{WindowRules: []WindowRule{{Class: "x", Sticky: true, Desktop: &one}}},
		{WorkspaceRules: []WorkspaceRule{{Desktop: 0}}},
		{WorkspaceRules: []WorkspaceRule{{Desktop: 1, Screen: &zero}}},
		{WorkspaceRules: []WorkspaceRule{{Desktop: 1, Layout: "unknown"}}},
	}
	for _, config := range tests {
		if err := validateConfig(&config); err == nil {
			t.Fatalf("expected invalid rule configuration to be rejected: %#v", config)
		}
	}
}

func TestNotifyConfigChangeInvokesRegisteredCallbacks(t *testing.T) {
	configChangeMu.Lock()
	previous := configChangeCallbacks
	configChangeCallbacks = nil
	configChangeMu.Unlock()
	t.Cleanup(func() {
		configChangeMu.Lock()
		configChangeCallbacks = previous
		configChangeMu.Unlock()
	})

	called := false
	OnConfigChange(func() {
		called = true
	})
	notifyConfigChange()
	if !called {
		t.Fatal("config-change callback was not invoked")
	}
}
