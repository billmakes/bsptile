// Package proto carries the wire types for the bsptile control socket.
// It is shared by the daemon and the bsptilectl client, so it must not
// import anything from the rest of the project.
package proto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Command names.
const (
	CmdAction    = "action"
	CmdQuery     = "query"
	CmdSubscribe = "subscribe"
	CmdReload    = "reload"
	CmdWM        = "wm"
)

// Subscription topics.
const (
	TopicAction     = "action"
	TopicWorkplace  = "workplace"
	TopicWindows    = "windows"
	TopicClients    = "clients"
	TopicWorkspaces = "workspaces"
	TopicAll        = "*"
)

// Query targets.
const (
	QueryWorkspaces = "workspaces"
	QueryWindows    = "windows"
	QueryClients    = "clients"
	QueryWorkplace  = "workplace"
	QueryConfig     = "config"
)

// Window-manager operations.
const (
	WMExit    = "exit"
	WMRestart = "restart"
)

// Request is one socket call. Cmd selects which other fields are read.
type Request struct {
	Cmd    string   `json:"cmd"`
	Name   string   `json:"name,omitempty"`
	Mod    string   `json:"mod,omitempty"`
	Target string   `json:"target,omitempty"`
	Topics []string `json:"topics,omitempty"`
	Op     string   `json:"op,omitempty"`
}

// Response is the reply to a non-subscribe request.
type Response struct {
	OK    bool        `json:"ok"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// Event is a single push message on a subscribe connection.
type Event struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data,omitempty"`
}

// SocketPath resolves the daemon's listen address. Precedence:
// $BSPTILE_SOCKET → $XDG_RUNTIME_DIR/bsptile-<display>.sock → /tmp/bsptile-<display>-<uid>.sock.
func SocketPath() string {
	if env := os.Getenv("BSPTILE_SOCKET"); env != "" {
		return env
	}
	display := strings.TrimPrefix(os.Getenv("DISPLAY"), ":")
	if display == "" {
		display = "0"
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, fmt.Sprintf("bsptile-%s.sock", display))
	}
	return fmt.Sprintf("/tmp/bsptile-%s-%d.sock", display, os.Getuid())
}
