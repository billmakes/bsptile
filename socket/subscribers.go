package socket

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/billmakes/bsptile/v2/socket/proto"
)

// Subscribers tracks open subscribe connections and which topics each cares
// about. Publish is safe for concurrent callers; writes are best-effort and
// drop subscribers whose socket errors.
type Subscribers struct {
	mu    sync.Mutex
	conns map[net.Conn]*subscription
}

type subscription struct {
	conn   net.Conn
	topics map[string]bool
	queue  chan []byte
	done   chan struct{}
	once   sync.Once
}

func NewSubscribers() *Subscribers {
	return &Subscribers{conns: make(map[net.Conn]*subscription)}
}

func (s *Subscribers) Add(conn net.Conn, topics []string) {
	set := make(map[string]bool, len(topics))
	for _, t := range topics {
		set[t] = true
	}
	sub := &subscription{
		conn:   conn,
		topics: set,
		queue:  make(chan []byte, 16),
		done:   make(chan struct{}),
	}
	s.mu.Lock()
	s.conns[conn] = sub
	s.mu.Unlock()
	go s.writeLoop(sub)
}

func (s *Subscribers) Remove(conn net.Conn) {
	s.mu.Lock()
	sub := s.conns[conn]
	if sub != nil {
		delete(s.conns, conn)
	}
	s.mu.Unlock()
	if sub != nil {
		sub.close()
	}
}

func (s *Subscribers) Close() {
	s.mu.Lock()
	subscriptions := make([]*subscription, 0, len(s.conns))
	for conn, sub := range s.conns {
		delete(s.conns, conn)
		subscriptions = append(subscriptions, sub)
	}
	s.mu.Unlock()
	for _, sub := range subscriptions {
		sub.close()
	}
}

// Publish writes the event to every connection whose subscription matches
// the topic (or that subscribed to "*"). Errors close and drop the conn.
func (s *Subscribers) Publish(topic string, payload interface{}) {
	data, err := json.Marshal(proto.Event{Event: topic, Data: payload})
	if err != nil {
		return
	}
	data = append(data, '\n')

	var dropped []*subscription
	s.mu.Lock()
	for conn, sub := range s.conns {
		if !sub.topics[topic] && !sub.topics[proto.TopicAll] {
			continue
		}
		select {
		case sub.queue <- data:
		default:
			delete(s.conns, conn)
			dropped = append(dropped, sub)
		}
	}
	s.mu.Unlock()
	for _, sub := range dropped {
		sub.close()
	}
}

func (s *Subscribers) writeLoop(sub *subscription) {
	for {
		select {
		case data := <-sub.queue:
			sub.conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
			_, err := sub.conn.Write(data)
			sub.conn.SetWriteDeadline(time.Time{})
			if err != nil {
				s.Remove(sub.conn)
				return
			}
		case <-sub.done:
			return
		}
	}
}

func (sub *subscription) close() {
	sub.once.Do(func() {
		close(sub.done)
		sub.conn.Close()
	})
}
