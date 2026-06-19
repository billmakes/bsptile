package store

import (
	"fmt"

	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgbutil/ewmh"

	log "github.com/sirupsen/logrus"
)

func RequestWindowState(window xproto.Window, action int, states ...string) bool {
	if len(states) == 0 {
		return true
	}
	for _, state := range states {
		if err := ewmh.WmStateReq(X, window, action, state); err != nil {
			log.Warn("Error requesting window state ", state, " for ", window, ": ", err)
			return false
		}
	}
	return true
}

func SetWindowDesktop(window xproto.Window, desktop uint32) bool {
	if err := ewmh.WmDesktopSet(X, window, uint(desktop)); err != nil {
		log.Warn("Error setting window desktop for ", window, ": ", err)
		return false
	}
	if err := ewmh.ClientEvent(X, window, "_NET_WM_DESKTOP", int(desktop), int(2)); err != nil {
		log.Warn("Error requesting window desktop for ", window, ": ", err)
		return false
	}
	return true
}

func MoveXWindow(window xproto.Window, x, y int) bool {
	if err := ewmh.MoveWindow(X, window, x, y); err != nil {
		log.Warn("Error moving window ", window, ": ", err)
		return false
	}
	return true
}

func MoveResizeXWindow(window xproto.Window, x, y, width, height int) bool {
	if width <= 0 || height <= 0 {
		log.Warn("Reject invalid window geometry for ", window, ": ", width, "x", height)
		return false
	}
	if err := ewmh.MoveresizeWindow(X, window, x, y, width, height); err != nil {
		log.Warn("Error moving/resizing window ", window, ": ", err)
		return false
	}
	return true
}

func RequestActiveWindow(window xproto.Window) bool {
	if err := ewmh.ActiveWindowReq(X, window); err != nil {
		log.Warn("Error requesting active window ", window, ": ", err)
		return false
	}
	return true
}

func CloseXWindow(window xproto.Window) bool {
	if err := ewmh.CloseWindow(X, window); err != nil {
		log.Warn("Error requesting window close for ", window, ": ", err)
		return false
	}
	return true
}

func SetCurrentDesktop(desktop uint) bool {
	if err := ewmh.CurrentDesktopSet(X, desktop); err != nil {
		log.Warn("Error setting current desktop ", desktop, ": ", err)
		return false
	}
	if err := ewmh.ClientEvent(X, X.RootWin(), "_NET_CURRENT_DESKTOP", int(desktop), int(0)); err != nil {
		log.Warn("Error requesting current desktop ", desktop, ": ", err)
		return false
	}
	return true
}

func WindowStateActionName(action int) string {
	switch action {
	case ewmh.StateRemove:
		return "remove"
	case ewmh.StateAdd:
		return "add"
	case ewmh.StateToggle:
		return "toggle"
	default:
		return fmt.Sprintf("%d", action)
	}
}
