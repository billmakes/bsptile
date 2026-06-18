// Package socket exposes a Unix-domain control socket for bsptile.
// The wire types live in socket/proto so cmd/bsptilectl can import them
// without dragging in the rest of the daemon.
package socket

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"

	"golang.org/x/exp/maps"

	"github.com/billmakes/bsptile/v2/common"
	"github.com/billmakes/bsptile/v2/desktop"
	"github.com/billmakes/bsptile/v2/input"
	"github.com/billmakes/bsptile/v2/socket/proto"
	"github.com/billmakes/bsptile/v2/store"

	log "github.com/sirupsen/logrus"
)

// Server owns the listener and active subscribers.
type Server struct {
	Tracker  *desktop.Tracker
	Path     string
	listener net.Listener
	subs     *Subscribers
}

// Internal event names from tr.Channels.Event mapped to public topic names.
var topicForEvent = map[string]string{
	"workplace_change":  proto.TopicWorkplace,
	"windows_change":    proto.TopicWindows,
	"clients_change":    proto.TopicClients,
	"workspaces_change": proto.TopicWorkspaces,
}

// Init starts the control socket. The path comes from --socket, then
// $BSPTILE_SOCKET, then the SocketPath default. Returns a server even on
// failure so callers can still log the error path.
func Init(tr *desktop.Tracker) (*Server, error) {
	path := common.Args.Socket
	if path == "" {
		path = proto.SocketPath()
	}

	// Stale socket from a previous run blocks Listen; remove it first.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warn("Error removing stale socket: ", err)
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	s := &Server{Tracker: tr, Path: path, listener: l, subs: NewSubscribers()}

	// Fan out tracker events to subscribers.
	desktop.OnEvent(func(e string) {
		topic, ok := topicForEvent[e]
		if !ok {
			return
		}
		s.subs.Publish(topic, s.payloadForTopic(topic))
	})

	// Fan out executed actions to subscribers.
	input.OnExecute(func(action string, desk uint, screen uint) {
		s.subs.Publish(proto.TopicAction, map[string]interface{}{
			"name":    action,
			"desktop": desk,
			"screen":  screen,
		})
	})

	log.Info("Control socket listening on ", path)
	go s.accept()
	return s, nil
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener closed during shutdown.
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	keepOpen := false
	defer func() {
		if !keepOpen {
			conn.Close()
		}
	}()

	var req proto.Request
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&req); err != nil {
		writeResponse(conn, proto.Response{Error: "invalid request: " + err.Error()})
		return
	}

	switch req.Cmd {
	case proto.CmdAction:
		s.handleAction(conn, req)
	case proto.CmdQuery:
		s.handleQuery(conn, req)
	case proto.CmdSubscribe:
		keepOpen = s.handleSubscribe(conn, req)
	case proto.CmdReload:
		writeResponse(conn, proto.Response{OK: input.ReloadConfig(s.Tracker)})
	case proto.CmdWM:
		s.handleWM(conn, req)
	default:
		writeResponse(conn, proto.Response{Error: "unknown command: " + req.Cmd})
	}
}

func (s *Server) handleAction(conn net.Conn, req proto.Request) {
	if req.Name == "" {
		writeResponse(conn, proto.Response{Error: "missing action name"})
		return
	}
	mod := req.Mod
	if mod == "" {
		mod = "current"
	}
	ok := input.ExecuteActions(req.Name, s.Tracker, mod)
	writeResponse(conn, proto.Response{OK: ok})
}

func (s *Server) handleQuery(conn net.Conn, req proto.Request) {
	target := req.Target
	if target == "" {
		writeResponse(conn, proto.Response{OK: true, Data: map[string]interface{}{
			proto.QueryWorkspaces: maps.Values(s.Tracker.Workspaces),
			proto.QueryWindows:    store.Windows,
			proto.QueryClients:    maps.Values(s.Tracker.Clients),
			proto.QueryWorkplace:  store.Workplace,
			proto.QueryConfig:     common.Config,
		}})
		return
	}
	data := s.payloadForTopic(target)
	if data == nil {
		writeResponse(conn, proto.Response{Error: "unknown query target: " + target})
		return
	}
	writeResponse(conn, proto.Response{OK: true, Data: data})
}

// payloadForTopic returns the live snapshot used both for query responses
// and for the payload of a subscription push. Topic and query target strings
// share the same values, so one switch covers both callers. Maps are
// flattened to slices since encoding/json can't marshal struct-keyed maps.
func (s *Server) payloadForTopic(name string) interface{} {
	switch name {
	case proto.TopicWorkspaces:
		return maps.Values(s.Tracker.Workspaces)
	case proto.TopicWindows:
		return store.Windows
	case proto.TopicClients:
		return maps.Values(s.Tracker.Clients)
	case proto.TopicWorkplace:
		return store.Workplace
	case proto.QueryConfig:
		return common.Config
	}
	return nil
}

func (s *Server) handleSubscribe(conn net.Conn, req proto.Request) bool {
	topics := req.Topics
	if len(topics) == 0 {
		topics = []string{proto.TopicAll}
	}
	if err := writeResponse(conn, proto.Response{OK: true}); err != nil {
		return false
	}
	s.subs.Add(conn, topics)

	// Hold the goroutine until the client disconnects. A blocking Read on a
	// well-behaved client never returns until the client closes; an error
	// here means we should drop the subscription and let the deferred close
	// run.
	go func() {
		defer s.subs.Remove(conn)
		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				if !errors.Is(err, io.EOF) {
					log.Debug("Subscriber read error: ", err)
				}
				return
			}
		}
	}()

	return true
}

func (s *Server) handleWM(conn net.Conn, req proto.Request) {
	switch req.Op {
	case proto.WMRestart:
		writeResponse(conn, proto.Response{OK: true})
		input.Restart(s.Tracker)
	case proto.WMExit:
		writeResponse(conn, proto.Response{OK: true})
		input.Exit(s.Tracker)
	default:
		writeResponse(conn, proto.Response{Error: "unknown wm op: " + req.Op})
	}
}

// Close stops accepting and removes the socket file. Active subscribers
// stay open; their goroutines will exit when the client disconnects.
func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.Path != "" {
		os.Remove(s.Path)
	}
}

func writeResponse(conn net.Conn, resp proto.Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = conn.Write(data)
	return err
}
