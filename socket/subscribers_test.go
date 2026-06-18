package socket

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/billmakes/bsptile/v2/socket/proto"
)

func TestSubscribersPublishesOnlyMatchingTopics(t *testing.T) {
	subs := NewSubscribers()
	clientWorkplace, serverWorkplace := net.Pipe()
	clientAction, serverAction := net.Pipe()
	defer clientWorkplace.Close()
	defer clientAction.Close()

	subs.Add(serverWorkplace, []string{proto.TopicWorkplace})
	subs.Add(serverAction, []string{proto.TopicAction})

	// net.Pipe is synchronous: Publish.Write blocks until a reader appears.
	go subs.Publish(proto.TopicWorkplace, map[string]int{"desktop": 1})

	if line, err := readLine(clientWorkplace); err != nil {
		t.Fatalf("workplace subscriber read failed: %v", err)
	} else if !strings.Contains(line, `"event":"workplace"`) {
		t.Fatalf("workplace subscriber got %q", line)
	}

	// The action subscriber should see nothing on the workplace topic.
	clientAction.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := bufio.NewReader(clientAction).ReadByte(); err == nil {
		t.Fatal("action subscriber received an event it did not subscribe to")
	}
}

func TestSubscribersWildcardReceivesEverything(t *testing.T) {
	subs := NewSubscribers()
	client, server := net.Pipe()
	defer client.Close()
	subs.Add(server, []string{proto.TopicAll})

	go subs.Publish(proto.TopicAction, "payload")

	line, err := readLine(client)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var ev proto.Event
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatalf("decode failed: %v (line=%q)", err, line)
	}
	if ev.Event != proto.TopicAction {
		t.Fatalf("event = %q, want %q", ev.Event, proto.TopicAction)
	}
}

func TestSubscribersDropsBrokenConnections(t *testing.T) {
	subs := NewSubscribers()
	client, server := net.Pipe()

	subs.Add(server, []string{proto.TopicAll})
	client.Close() // kill the reader side

	// Publish should detect the dead conn and remove the subscriber.
	subs.Publish(proto.TopicAction, "payload")

	subs.mu.Lock()
	_, present := subs.conns[server]
	subs.mu.Unlock()
	if present {
		t.Fatal("broken subscriber was not removed")
	}
}

func TestSubscribersRemove(t *testing.T) {
	subs := NewSubscribers()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	subs.Add(server, []string{proto.TopicAction})
	subs.Remove(server)

	subs.mu.Lock()
	_, present := subs.conns[server]
	subs.mu.Unlock()
	if present {
		t.Fatal("subscriber should have been removed")
	}
}

func readLine(conn net.Conn) (string, error) {
	conn.SetReadDeadline(time.Now().Add(time.Second))
	return bufio.NewReader(conn).ReadString('\n')
}
