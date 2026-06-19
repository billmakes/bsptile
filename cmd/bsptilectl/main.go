// bsptilectl is the command-line client for the bsptile control socket.
// It mirrors bspc's role for bspwm: open the Unix socket, send a single
// JSON request, print the response. For "subscribe" it keeps the socket
// open and streams JSON events to stdout until the daemon hangs up.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/billmakes/bsptile/v2/socket/proto"
)

func main() {
	var socketPath string
	fs := flag.NewFlagSet("bsptilectl", flag.ContinueOnError)
	fs.StringVar(&socketPath, "socket", "", "control socket path")
	fs.Usage = usage
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := fs.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	if socketPath == "" {
		socketPath = proto.SocketPath()
	}

	req, stream, err := buildRequest(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bsptilectl:", err)
		os.Exit(2)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bsptilectl: cannot connect to", socketPath+":", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		fmt.Fprintln(os.Stderr, "bsptilectl: send failed:", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "bsptilectl: read failed:", err)
		os.Exit(1)
	}
	if len(line) > 0 {
		os.Stdout.Write(line)
	}

	// Decode just enough to set the exit code; ignore parse errors so we
	// don't mask a bad response by returning success.
	var resp proto.Response
	if jerr := json.Unmarshal(line, &resp); jerr != nil || !resp.OK {
		os.Exit(1)
	}

	if !stream {
		return
	}

	// Stream subscription events until the daemon hangs up.
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			os.Stdout.Write(line)
		}
		if err != nil {
			return
		}
	}
}

func buildRequest(args []string) (proto.Request, bool, error) {
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case proto.QueryActions:
		if len(rest) != 0 {
			return proto.Request{}, false, fmt.Errorf("actions: unexpected args")
		}
		return proto.Request{Cmd: proto.CmdQuery, Target: proto.QueryActions}, false, nil

	case proto.CmdAction:
		if len(rest) == 0 {
			return proto.Request{}, false, fmt.Errorf("action: missing name")
		}
		if len(rest) == 1 && (rest[0] == "--list" || rest[0] == "list") {
			return proto.Request{Cmd: proto.CmdQuery, Target: proto.QueryActions}, false, nil
		}
		req := proto.Request{Cmd: proto.CmdAction, Name: rest[0]}
		for i := 1; i < len(rest); i++ {
			if rest[i] != "--mod" {
				return proto.Request{}, false, fmt.Errorf("action: unexpected arg %q", rest[i])
			}
			if i+1 >= len(rest) {
				return proto.Request{}, false, fmt.Errorf("action: --mod requires a value")
			}
			req.Mod = rest[i+1]
			i++
		}
		return req, false, nil

	case proto.CmdQuery:
		req := proto.Request{Cmd: proto.CmdQuery}
		if len(rest) > 0 {
			req.Target = rest[0]
		}
		return req, false, nil

	case proto.CmdSubscribe:
		return proto.Request{Cmd: proto.CmdSubscribe, Topics: rest}, true, nil

	case proto.CmdReload:
		return proto.Request{Cmd: proto.CmdReload}, false, nil

	case proto.CmdWM:
		if len(rest) == 0 {
			return proto.Request{}, false, fmt.Errorf("wm: missing op")
		}
		return proto.Request{Cmd: proto.CmdWM, Op: rest[0]}, false, nil
	}

	return proto.Request{}, false, fmt.Errorf("unknown command: %s", cmd)
}

func usage() {
	fmt.Fprintln(os.Stderr, `bsptilectl - bsptile control client

Usage:
  bsptilectl [--socket PATH] <command> [args...]

Commands:
  action <name> [--mod current|screens|workspaces]
  action --list
  actions
  query [workspaces|windows|clients|workplace|config]
  subscribe [topic...]
  reload
  wm exit|restart

Subscription topics:
  action, workplace, windows, clients, workspaces, *

Environment:
  BSPTILE_SOCKET   override the default socket path`)
}
