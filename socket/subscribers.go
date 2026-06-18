package socket

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/billmakes/bsptile/v2/socket/proto"
)

// Subscribers tracks open subscribe connections and which topics each cares
// about. Publish is safe for concurrent callers; writes are best-effort and
// drop subscribers whose socket errors.
type Subscribers struct {
	mu    sync.Mutex
	conns map[net.Conn]map[string]bool
}

func NewSubscribers() *Subscribers {
	return &Subscribers{conns: make(map[net.Conn]map[string]bool)}
}

func (s *Subscribers) Add(conn net.Conn, topics []string) {
	set := make(map[string]bool, len(topics))
	for _, t := range topics {
		set[t] = true
	}
	s.mu.Lock()
	s.conns[conn] = set
	s.mu.Unlock()
}

func (s *Subscribers) Remove(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// Publish writes the event to every connection whose subscription matches
// the topic (or that subscribed to "*"). Errors close and drop the conn.
func (s *Subscribers) Publish(topic string, payload interface{}) {
	data, err := json.Marshal(proto.Event{Event: topic, Data: payload})
	if err != nil {
		return
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	for conn, topics := range s.conns {
		if !topics[topic] && !topics[proto.TopicAll] {
			continue
		}
		if _, err := conn.Write(data); err != nil {
			conn.Close()
			delete(s.conns, conn)
		}
	}
}
