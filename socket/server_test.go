package socket

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/billmakes/bsptile/v2/socket/proto"
)

func TestServerHandleRejectsUnknownCommand(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := sendRequest(t, srv.Path, proto.Request{Cmd: "bogus"})
	if resp.OK {
		t.Fatal("expected ok=false for unknown command")
	}
	if !strings.Contains(resp.Error, "unknown command") {
		t.Fatalf("error = %q, want unknown-command message", resp.Error)
	}
}

func TestServerHandleRejectsInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	conn, err := net.Dial("unix", srv.Path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.Write([]byte("not json\n"))

	conn.SetReadDeadline(time.Now().Add(time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var resp proto.Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode: %v (line=%q)", err, line)
	}
	if resp.OK {
		t.Fatal("expected ok=false for invalid JSON")
	}
	if !strings.Contains(resp.Error, "invalid request") {
		t.Fatalf("error = %q, want invalid-request message", resp.Error)
	}
}

func TestValidActionModifier(t *testing.T) {
	for _, modifier := range []string{"current", "screens", "workspaces"} {
		if !validActionModifier(modifier) {
			t.Fatalf("valid modifier %q was rejected", modifier)
		}
	}
	if validActionModifier("bogus") {
		t.Fatal("invalid modifier was accepted")
	}
}

func TestServerQueryActions(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := sendRequest(t, srv.Path, proto.Request{Cmd: proto.CmdQuery, Target: proto.QueryActions})
	if !resp.OK {
		t.Fatalf("query actions failed: %s", resp.Error)
	}
	actions, ok := resp.Data.([]interface{})
	if !ok || len(actions) == 0 {
		t.Fatalf("actions payload = %#v, want non-empty array", resp.Data)
	}
	foundToggle := false
	foundDesktopPattern := false
	foundDesktopSwitchPattern := false
	for _, entry := range actions {
		action, ok := entry.(map[string]interface{})
		if !ok {
			t.Fatalf("action entry = %#v, want object", entry)
		}
		switch action["name"] {
		case "close":
			if action["description"] == "" {
				t.Fatal("close action is missing description")
			}
		case "toggle":
			foundToggle = true
			if action["description"] == "" {
				t.Fatal("toggle action is missing description")
			}
		case "window_to_desktop_<n>":
			foundDesktopPattern = true
		case "desktop_<n>":
			foundDesktopSwitchPattern = true
		}
	}
	if !foundToggle {
		t.Fatal("actions payload missing toggle")
	}
	if !foundDesktopPattern {
		t.Fatal("actions payload missing dynamic desktop-send pattern")
	}
	if !foundDesktopSwitchPattern {
		t.Fatal("actions payload missing dynamic desktop-switch pattern")
	}
}

func TestServerSubscribeAcksThenStreamsEvents(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	conn, err := net.Dial("unix", srv.Path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(proto.Request{
		Cmd:    proto.CmdSubscribe,
		Topics: []string{proto.TopicAction},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(time.Second))

	// First line: subscribe ack.
	ackLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	var ack proto.Response
	json.Unmarshal([]byte(ackLine), &ack)
	if !ack.OK {
		t.Fatalf("subscribe ack not ok: %s", ackLine)
	}

	// Give the server time to register the subscription before publishing.
	waitForSubscriber(t, srv, time.Second)

	srv.subs.Publish(proto.TopicAction, map[string]string{"name": "balance"})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	evLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	var ev proto.Event
	if err := json.Unmarshal([]byte(evLine), &ev); err != nil {
		t.Fatalf("decode event: %v (line=%q)", err, evLine)
	}
	if ev.Event != proto.TopicAction {
		t.Fatalf("event = %q, want %q", ev.Event, proto.TopicAction)
	}
}

func TestRemoveStaleSocketRejectsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bsptile.sock")
	if err := os.WriteFile(path, []byte("do not delete"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err == nil {
		t.Fatal("expected regular file at socket path to be rejected")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}
}

func TestRemoveStaleSocketRemovesOwnedSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bsptile.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()

	if err := removeStaleSocket(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("stale socket still exists: %v", err)
	}
}

func TestSocketPermissionsArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bsptile.sock")
	listener, err := listenControlSocket(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
}

// newTestServer starts a Server bound to a tempdir socket with no tracker.
// Bypasses Init() so tests don't depend on input/desktop/store globals.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bsptile.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &Server{Path: path, listener: l, subs: NewSubscribers()}
	go srv.accept()
	return srv
}

func sendRequest(t *testing.T, path string, req proto.Request) proto.Response {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var resp proto.Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode: %v (line=%q)", err, line)
	}
	return resp
}

func waitForSubscriber(t *testing.T, srv *Server, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		srv.subs.mu.Lock()
		n := len(srv.subs.conns)
		srv.subs.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for subscriber to register")
}
