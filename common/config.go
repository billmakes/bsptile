package common

import (
	"fmt"
	"os"

	"encoding/json"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/fsnotify/fsnotify"

	log "github.com/sirupsen/logrus"
)

var (
	Config Configuration // Decoded config values
)

type Configuration struct {
	TilingEnabled       bool                   `toml:"tiling_enabled"`        // Tile windows on startup
	TilingGui           int                    `toml:"tiling_gui"`            // Time duration of gui
	TilingIcon          [][]string             `toml:"tiling_icon"`           // Menu entries of systray
	KeybindingsEnabled  bool                   `toml:"keybindings_enabled"`   // Register built-in global keyboard shortcuts
	WindowIgnore        [][]string             `toml:"window_ignore"`         // Regex to ignore windows
	WindowGapSize       int                    `toml:"window_gap_size"`       // Gap size between windows
	WindowFocusDelay    int                    `toml:"window_focus_delay"`    // Window focus delay when hovered
	WindowPointerWarp   bool                   `toml:"window_pointer_warp"`   // Move pointer with keyboard window actions
	WindowFloatingAbove bool                   `toml:"window_floating_above"` // Keep eligible unmanaged windows above tiled windows
	WindowDecoration    bool                   `toml:"window_decoration"`     // Show window decorations
	ProportionStep      float64                `toml:"proportion_step"`       // BSP split ratio step
	ProportionMin       float64                `toml:"proportion_min"`        // BSP split ratio minimum
	EdgeMargin          []int                  `toml:"edge_margin"`           // Margin values of tiling area
	EdgeMarginPrimary   []int                  `toml:"edge_margin_primary"`   // Margin values of primary tiling area
	DropTargetWidth     int                    `toml:"drop_target_width"`     // Outline width (px) of drop-target indicator
	Colors              map[string][]int       `toml:"colors"`                // List of color values for gui elements
	Keys                map[string]KeyBindings `toml:"keys"`                  // Event bindings for keyboard shortcuts
	Mouse               map[string]KeyBindings `toml:"mouse"`                 // Event bindings for mouse buttons
	Modes               map[string]Mode        `toml:"modes"`                 // Alternate keyboard shortcut layers
	Systray             map[string]string      `toml:"systray"`               // Event bindings for systray icon
	WindowRules         []WindowRule           `toml:"window_rules"`          // Per-class/name overrides applied when a window is first tracked
	WorkspaceRules      []WorkspaceRule        `toml:"workspace_rules"`       // Per-workspace initial state overrides
}

// WindowRule applies overrides to a freshly tracked window whose WM_CLASS
// matches Class and (if set) WM_NAME matches Name — both as RE2 regex.
// First match wins; later rules are ignored.
//
// Indexing in Monitor/Desktop is 1-based to match what xfwm shows in its UI
// (Workspace 1, Workspace 2, …). Internally we convert to bsptile's 0-based
// desktop/screen indices at the boundary.
type WindowRule struct {
	Class    string `toml:"class"`              // required: regex for WM_CLASS
	Name     string `toml:"name,omitempty"`     // optional: regex for WM_NAME
	Floating bool   `toml:"floating,omitempty"` // true → leave unmanaged (same as window_ignore)
	Tile     bool   `toml:"tile,omitempty"`     // true → force-tile even when IsFloating() would say no
	Monitor  *int   `toml:"monitor,omitempty"`  // optional: send to this monitor (1-indexed)
	Desktop  *int   `toml:"desktop,omitempty"`  // optional: send to this desktop (1-indexed)
}

// WorkspaceRule sets the initial state of one (or every) workspace on a given
// desktop. Desktop and Screen are 1-indexed to match the xfwm UI.
//
// Anything you change at runtime (toggle, layout switch, …) still wins until
// the daemon next restarts or reloads.
type WorkspaceRule struct {
	Desktop    int    `toml:"desktop"`              // required: 1-indexed desktop number
	Screen     *int   `toml:"screen,omitempty"`     // optional: 1-indexed; absent = all screens
	Tiling     *bool  `toml:"tiling,omitempty"`     // override tiling_enabled
	Layout     string `toml:"layout,omitempty"`     // "bsp" | "maximized" | "fullscreen"
	Decoration *bool  `toml:"decoration,omitempty"` // override window_decoration
}

type KeyBindings []string
type Mode map[string]KeyBindings

func (keys *KeyBindings) UnmarshalTOML(value any) error {
	switch value := value.(type) {
	case string:
		*keys = KeyBindings{value}
	case []any:
		bindings := make(KeyBindings, 0, len(value))
		for _, item := range value {
			binding, ok := item.(string)
			if !ok {
				return fmt.Errorf("key binding must be a string")
			}
			bindings = append(bindings, binding)
		}
		*keys = bindings
	default:
		return fmt.Errorf("key binding must be a string or array of strings")
	}

	return nil
}

func InitConfig() {

	// Create config folder if not exists
	configFolderPath := filepath.Dir(Args.Config)
	if _, err := os.Stat(configFolderPath); os.IsNotExist(err) {
		os.MkdirAll(configFolderPath, 0755)
	}

	// Write default config if not exists
	if _, err := os.Stat(Args.Config); os.IsNotExist(err) {
		os.WriteFile(Args.Config, File.Toml, 0644)
	}

	// Read config file into memory
	readConfig(Args.Config, true)

	// Config file system watcher
	watchConfig(Args.Config)
}

func ReloadConfig() bool {
	return readConfig(Args.Config, false)
}

func ConfigFolderPath(name string) string {

	// Obtain user config directory
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal("Error obtaining config directory: ", err)
	}

	return filepath.Join(userConfigDir, name)
}

func readConfig(configFilePath string, initial bool) bool {

	// Print runtime infos
	if initial {
		fmt.Print("BUILD")
		if HasReleaseInfos() {
			fmt.Printf(" [>>> %s v%s is available <<<]", Build.Name, Source.Releases[0].Name)
		}
		fmt.Printf(": \n  name: %s\n  target: %s\n  version: v%s-%s\n  date: %s\n  flags: %s\n\n", Build.Name, Build.Target, Build.Version, Build.Commit, Build.Date, Build.Flags)
		fmt.Printf("FILES: \n  log: %s\n  lock: %s\n  cache: %s\n  config: %s\n\n", Args.Log, Args.Lock, Args.Cache, configFilePath)
	}

	// Decode into a temporary value so an invalid file cannot partially update
	// the running configuration.
	config := Configuration{KeybindingsEnabled: true}
	_, err := toml.DecodeFile(configFilePath, &config)
	if err != nil {
		if initial {
			log.Fatal("Error reading config file ", err)
		} else {
			log.Warn("Error updating config file ", err)
		}
		return false
	}
	if err := validateConfig(config); err != nil {
		if initial {
			log.Fatal("Error reading config file ", err)
		} else {
			log.Warn("Error updating config file ", err)
		}
		return false
	}
	Config = config

	// Print shortcut infos
	if initial {
		keys, _ := json.MarshalIndent(Config.Keys, "", "  ")
		mouse, _ := json.MarshalIndent(Config.Mouse, "", "  ")
		systray, _ := json.MarshalIndent(Config.Systray, "", "  ")

		fmt.Printf("KEYS: %s\n", RemoveChars(string(keys), []string{"{", "}", "\"", ","}))
		fmt.Printf("MOUSE: %s\n", RemoveChars(string(mouse), []string{"{", "}", "\"", ","}))
		fmt.Printf("SYSTRAY: %s\n", RemoveChars(string(systray), []string{"{", "}", "\"", ","}))
	}

	return true
}

func validateConfig(config Configuration) error {
	for name, mode := range config.Modes {
		if name == "" || name == "default" {
			return fmt.Errorf("invalid key mode name %q", name)
		}
		if bindings := mode["mode_default"]; len(bindings) == 0 {
			return fmt.Errorf("key mode %q must define mode_default", name)
		}
	}

	return nil
}

func watchConfig(configFilePath string) {

	// Init file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err)
	} else {
		watcher.Add(configFilePath)
	}

	// Listen for events
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					readConfig(configFilePath, false)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error(err)
			}
		}
	}()
}
