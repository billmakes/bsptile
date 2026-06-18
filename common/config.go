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
	TilingEnabled     bool                   `toml:"tiling_enabled"`      // Tile windows on startup
	TilingGui         int                    `toml:"tiling_gui"`          // Time duration of gui
	TilingIcon        [][]string             `toml:"tiling_icon"`         // Menu entries of systray
	WindowIgnore      [][]string             `toml:"window_ignore"`       // Regex to ignore windows
	WindowGapSize     int                    `toml:"window_gap_size"`     // Gap size between windows
	WindowFocusDelay  int                    `toml:"window_focus_delay"`  // Window focus delay when hovered
	WindowPointerWarp bool                   `toml:"window_pointer_warp"` // Move pointer with keyboard window actions
	WindowDecoration  bool                   `toml:"window_decoration"`   // Show window decorations
	ProportionStep    float64                `toml:"proportion_step"`     // BSP split ratio step
	ProportionMin     float64                `toml:"proportion_min"`      // BSP split ratio minimum
	EdgeMargin        []int                  `toml:"edge_margin"`         // Margin values of tiling area
	EdgeMarginPrimary []int                  `toml:"edge_margin_primary"` // Margin values of primary tiling area
	EdgeCornerSize    int                    `toml:"edge_corner_size"`    // Size of square defining edge corners
	EdgeCenterSize    int                    `toml:"edge_center_size"`    // Length of rectangle defining edge centers
	DropTargetWidth   int                    `toml:"drop_target_width"`   // Outline width (px) of drop-target indicator
	Colors            map[string][]int       `toml:"colors"`              // List of color values for gui elements
	Keys              map[string]KeyBindings `toml:"keys"`                // Event bindings for keyboard shortcuts
	Modes             map[string]Mode        `toml:"modes"`               // Alternate keyboard shortcut layers
	Corners           map[string]string      `toml:"corners"`             // Event bindings for hot-corner actions
	Systray           map[string]string      `toml:"systray"`             // Event bindings for systray icon
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
	var config Configuration
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
		corners, _ := json.MarshalIndent(Config.Corners, "", "  ")
		systray, _ := json.MarshalIndent(Config.Systray, "", "  ")

		fmt.Printf("KEYS: %s\n", RemoveChars(string(keys), []string{"{", "}", "\"", ","}))
		fmt.Printf("CORNERS: %s\n", RemoveChars(string(corners), []string{"{", "}", "\"", ","}))
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
