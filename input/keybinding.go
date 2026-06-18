package input

import (
	"strings"

	"github.com/jezek/xgbutil"
	"github.com/jezek/xgbutil/keybind"
	"github.com/jezek/xgbutil/xevent"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

const defaultKeyMode = "default"

var activeKeyMode = defaultKeyMode

func BindKeys(tr *desktop.Tracker) {
	keybind.Initialize(store.X)
	activeKeyMode = defaultKeyMode
	bindDefaultKeys(tr)

	// Bind action channel
	go action(tr.Channels.Action, tr)
}

func ReloadKeys(tr *desktop.Tracker) {
	setKeyMode(defaultKeyMode, tr)
}

func SetKeyMode(mode string, tr *desktop.Tracker) bool {
	if mode != defaultKeyMode {
		if _, ok := common.Config.Modes[mode]; !ok {
			log.Warn("Unknown key mode ", mode)
			return false
		}
	}

	setKeyMode(mode, tr)
	log.Info("Key mode ", mode)
	return true
}

func setKeyMode(mode string, tr *desktop.Tracker) {
	keybind.Detach(store.X, store.X.RootWin())
	activeKeyMode = mode
	if mode == defaultKeyMode {
		bindDefaultKeys(tr)
		return
	}
	bindModeKeys(common.Config.Modes[mode], tr)
}

func bindDefaultKeys(tr *desktop.Tracker) {
	actions := map[string]common.KeyBindings{}
	mods := map[string]common.KeyBindings{"current": {""}}

	// Map actions and modifiers
	for c, ck := range common.Config.Keys {
		if len(ck) == 0 {
			continue
		}
		if !strings.HasPrefix(c, "mod_") {
			actions[c] = ck
		} else {
			mods[c[4:]] = ck
		}
	}

	// Bind keyboard shortcuts
	for a, actionKeys := range actions {
		for _, ak := range actionKeys {
			if len(ak) == 0 {
				continue
			}
			for m, modifierKeys := range mods {
				for _, mk := range modifierKeys {
					if len(mk) == 0 {
						bind(ak, a, m, tr)
					} else {
						bind(mk+"-"+ak, a, m, tr)
					}
				}
			}
		}
	}
}

func bindModeKeys(mode common.Mode, tr *desktop.Tracker) {
	for action, keys := range mode {
		for _, key := range keys {
			if key != "" {
				bind(key, action, "current", tr)
			}
		}
	}
}

func keyModeAction(action string) (string, bool) {
	if action == "mode_default" {
		return defaultKeyMode, true
	}
	if strings.HasPrefix(action, "mode_") && len(action) > len("mode_") {
		return strings.TrimPrefix(action, "mode_"), true
	}
	return "", false
}

func bind(key string, action string, mod string, tr *desktop.Tracker) {
	err := keybind.KeyPressFun(func(X *xgbutil.XUtil, ev xevent.KeyPressEvent) {
		ExecuteActions(action, tr, mod)
	}).Connect(store.X, store.X.RootWin(), key, true)

	if err != nil {
		log.Warn("Error on action ", action, ": ", err)
	}
}

func action(ch chan string, tr *desktop.Tracker) {
	for {
		ExecuteAction(<-ch, tr, tr.ActiveWorkspace())
	}
}
