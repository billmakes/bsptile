package common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/fsnotify/fsnotify"

	log "github.com/sirupsen/logrus"
)

var (
	Config Configuration // Decoded config values

	configChangeMu        sync.Mutex
	configChangeCallbacks []func()
)

type Configuration struct {
	TilingEnabled       bool              `toml:"tiling_enabled"`        // Tile windows on startup
	TilingGui           int               `toml:"tiling_gui"`            // Time duration of gui
	TilingIcon          [][]string        `toml:"tiling_icon"`           // Menu entries of systray
	WindowIgnore        [][]string        `toml:"window_ignore"`         // Regex to ignore windows
	WindowGapSize       int               `toml:"window_gap_size"`       // Gap size between windows
	WindowFocusDelay    int               `toml:"window_focus_delay"`    // Window focus delay when hovered
	WindowPointerWarp   bool              `toml:"window_pointer_warp"`   // Move pointer with keyboard window actions
	WindowFloatingAbove bool              `toml:"window_floating_above"` // Keep eligible unmanaged windows above tiled windows
	WindowDecoration    bool              `toml:"window_decoration"`     // Show window decorations
	ProportionStep      float64           `toml:"proportion_step"`       // BSP split ratio step
	ProportionMin       float64           `toml:"proportion_min"`        // BSP split ratio minimum
	EdgeMargin          []int             `toml:"edge_margin"`           // Margin values of tiling area
	EdgeMarginPrimary   []int             `toml:"edge_margin_primary"`   // Margin values of primary tiling area
	DropTargetWidth     int               `toml:"drop_target_width"`     // Outline width (px) of drop-target indicator
	Colors              map[string][]int  `toml:"colors"`                // List of color values for gui elements
	Systray             map[string]string `toml:"systray"`               // Event bindings for systray icon
	WindowRules         []WindowRule      `toml:"window_rules"`          // Per-class/name overrides applied when a window is first tracked
	WorkspaceRules      []WorkspaceRule   `toml:"workspace_rules"`       // Per-workspace initial state overrides
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
	Sticky   bool   `toml:"sticky,omitempty"`   // true → float above and show on every desktop
	Tile     bool   `toml:"tile,omitempty"`     // true → force-tile even when IsFloating() would say no
	Monitor  *int   `toml:"monitor,omitempty"`  // optional: send to this monitor (1-indexed)
	Desktop  *int   `toml:"desktop,omitempty"`  // optional: send to this desktop (1-indexed)

	classPattern *regexp.Regexp
	namePattern  *regexp.Regexp
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

func InitConfig() {

	// Create config folder if not exists
	configFolderPath := filepath.Dir(Args.Config)
	if _, err := os.Stat(configFolderPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configFolderPath, 0755); err != nil {
			log.Fatal("Error creating config folder: ", err)
		}
	}

	// Write default config if not exists
	if _, err := os.Stat(Args.Config); os.IsNotExist(err) {
		if err := os.WriteFile(Args.Config, File.Toml, 0644); err != nil {
			log.Fatal("Error writing default config: ", err)
		}
	}

	// Read config file into memory
	readConfig(Args.Config, true)

	// Config file system watcher
	watchConfig(Args.Config)
}

func ReloadConfig() bool {
	return readConfig(Args.Config, false)
}

func OnConfigChange(fun func()) {
	configChangeMu.Lock()
	configChangeCallbacks = append(configChangeCallbacks, fun)
	configChangeMu.Unlock()
}

func notifyConfigChange() {
	configChangeMu.Lock()
	callbacks := append([]func(){}, configChangeCallbacks...)
	configChangeMu.Unlock()
	for _, fun := range callbacks {
		fun()
	}
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
	config := Configuration{}
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

	// Print action hook infos
	if initial {
		fmt.Printf("SYSTRAY: %v\n", Config.Systray)
	}

	return true
}

func validateConfig(config Configuration) error {
	for i, entry := range config.WindowIgnore {
		if len(entry) != 2 {
			return fmt.Errorf("window_ignore entry %d must contain class and name regexes", i+1)
		}
		if _, err := regexp.Compile(entry[0]); err != nil {
			return fmt.Errorf("window_ignore entry %d has invalid class regex: %w", i+1, err)
		}
		if entry[1] != "" {
			if _, err := regexp.Compile(entry[1]); err != nil {
				return fmt.Errorf("window_ignore entry %d has invalid name regex: %w", i+1, err)
			}
		}
	}

	for i, rule := range config.WindowRules {
		if rule.Class == "" {
			return fmt.Errorf("window_rules entry %d requires class", i+1)
		}
		classPattern, err := regexp.Compile(rule.Class)
		if err != nil {
			return fmt.Errorf("window_rules entry %d has invalid class regex: %w", i+1, err)
		}
		config.WindowRules[i].classPattern = classPattern
		if rule.Name != "" {
			namePattern, err := regexp.Compile(rule.Name)
			if err != nil {
				return fmt.Errorf("window_rules entry %d has invalid name regex: %w", i+1, err)
			}
			config.WindowRules[i].namePattern = namePattern
		}
		if rule.Tile && (rule.Floating || rule.Sticky) {
			return fmt.Errorf("window_rules entry %d cannot combine tile with floating or sticky", i+1)
		}
		if rule.Monitor != nil && *rule.Monitor < 1 {
			return fmt.Errorf("window_rules entry %d monitor must be at least 1", i+1)
		}
		if rule.Desktop != nil && *rule.Desktop < 1 {
			return fmt.Errorf("window_rules entry %d desktop must be at least 1", i+1)
		}
		if rule.Sticky && rule.Desktop != nil {
			return fmt.Errorf("window_rules entry %d cannot combine sticky with desktop", i+1)
		}
	}

	for i, rule := range config.WorkspaceRules {
		if rule.Desktop < 1 {
			return fmt.Errorf("workspace_rules entry %d desktop must be at least 1", i+1)
		}
		if rule.Screen != nil && *rule.Screen < 1 {
			return fmt.Errorf("workspace_rules entry %d screen must be at least 1", i+1)
		}
		if rule.Layout != "" && rule.Layout != "bsp" && rule.Layout != "maximized" && rule.Layout != "fullscreen" {
			return fmt.Errorf("workspace_rules entry %d has invalid layout %q", i+1, rule.Layout)
		}
	}

	return nil
}

func watchConfig(configFilePath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err)
		return
	}
	if err := watcher.Add(filepath.Dir(configFilePath)); err != nil {
		log.Error(err)
		watcher.Close()
		return
	}

	go func() {
		defer watcher.Close()
		var timer *time.Timer
		schedule := func() {
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(100*time.Millisecond, notifyConfigChange)
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != filepath.Clean(configFilePath) {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					schedule()
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
