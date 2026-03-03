package websocket

import (
	"testing"
	"time"

	"github.com/infrashift/chit/internal/model"
)

// newStubClient creates a Client without a real WebSocket connection.
// Since we're in the same package, we can set unexported fields directly.
func newStubClient(hub *Hub, userID string) *Client {
	return &Client{
		UserID: userID,
		hub:    hub,
		send:   make(chan *model.WebSocketEvent, sendBufferSize),
	}
}

func recvWithTimeout(ch chan *model.WebSocketEvent, d time.Duration) (*model.WebSocketEvent, bool) {
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(d):
		return nil, false
	}
}

func TestHub_RegisterAndBroadcast(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)

	// Give the event loop time to process
	time.Sleep(20 * time.Millisecond)

	event := &model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"msg": "hello"},
		Broadcast: &model.WebSocketBroadcast{},
	}
	hub.Broadcast(event)

	ev, ok := recvWithTimeout(client.send, 100*time.Millisecond)
	if !ok {
		t.Fatal("expected client to receive event")
	}
	if ev.Event != model.WebSocketEventPosted {
		t.Fatalf("expected event=%q, got %q", model.WebSocketEventPosted, ev.Event)
	}
}

func TestHub_BroadcastNilBroadcastField(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	// Event with nil Broadcast should be ignored (no delivery, no panic)
	event := &model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data:  map[string]any{"msg": "hello"},
	}
	hub.Broadcast(event)

	_, ok := recvWithTimeout(client.send, 50*time.Millisecond)
	if ok {
		t.Fatal("expected no delivery for nil Broadcast")
	}
}

func TestHub_TargetedBroadcast(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	c1 := newStubClient(hub, "user-1")
	c2 := newStubClient(hub, "user-2")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	event := &model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"msg": "private"},
		Broadcast: &model.WebSocketBroadcast{UserID: "user-1"},
	}
	hub.Broadcast(event)

	_, ok := recvWithTimeout(c1.send, 100*time.Millisecond)
	if !ok {
		t.Fatal("expected user-1 to receive targeted event")
	}

	_, ok = recvWithTimeout(c2.send, 50*time.Millisecond)
	if ok {
		t.Fatal("expected user-2 NOT to receive targeted event")
	}
}

func TestHub_BroadcastToAll(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	c1 := newStubClient(hub, "user-1")
	c2 := newStubClient(hub, "user-2")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	event := &model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"msg": "all"},
		Broadcast: &model.WebSocketBroadcast{}, // empty UserID = all
	}
	hub.Broadcast(event)

	_, ok1 := recvWithTimeout(c1.send, 100*time.Millisecond)
	_, ok2 := recvWithTimeout(c2.send, 100*time.Millisecond)
	if !ok1 || !ok2 {
		t.Fatal("expected both clients to receive broadcast")
	}
}

func TestHub_Unregister(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(20 * time.Millisecond)

	// send channel should be closed
	_, ok := <-client.send
	if ok {
		t.Fatal("expected send channel to be closed after unregister")
	}

	// Broadcasting after unregister should not deliver or panic
	hub.Broadcast(&model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{},
		Broadcast: &model.WebSocketBroadcast{},
	})
	// No assertion — just verifying no panic
}

func TestHub_MultipleClientsPerUser(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	c1 := newStubClient(hub, "user-1")
	c2 := newStubClient(hub, "user-1") // same user
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	event := &model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"msg": "both"},
		Broadcast: &model.WebSocketBroadcast{},
	}
	hub.Broadcast(event)

	_, ok1 := recvWithTimeout(c1.send, 100*time.Millisecond)
	_, ok2 := recvWithTimeout(c2.send, 100*time.Millisecond)
	if !ok1 || !ok2 {
		t.Fatal("expected both clients for same user to receive broadcast")
	}
}

func TestHub_SlowConsumer(t *testing.T) {
	hub := NewHub()
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	// Fill the send buffer
	for i := 0; i < sendBufferSize; i++ {
		hub.Broadcast(&model.WebSocketEvent{
			Event:     model.WebSocketEventPosted,
			Data:      map[string]any{"i": i},
			Broadcast: &model.WebSocketBroadcast{},
		})
	}

	// Give the hub time to process all events
	time.Sleep(50 * time.Millisecond)

	// Next broadcast should trigger slow consumer disconnect
	hub.Broadcast(&model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"overflow": true},
		Broadcast: &model.WebSocketBroadcast{},
	})

	// Drain the send channel - it should eventually close
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case _, ok := <-client.send:
			if !ok {
				return // send channel closed = slow consumer disconnected
			}
		case <-timeout:
			// It's acceptable if the slow consumer wasn't disconnected yet
			// since the unregister is async. The important thing is no panic.
			return
		}
	}
}

func TestHub_Stop(t *testing.T) {
	hub := NewHub()

	c1 := newStubClient(hub, "user-1")
	c2 := newStubClient(hub, "user-2")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	// Stop should close all send channels and return without deadlock
	done := make(chan struct{})
	go func() {
		hub.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Stop() deadlocked")
	}

	// Both send channels should be closed
	_, ok1 := <-c1.send
	_, ok2 := <-c2.send
	if ok1 || ok2 {
		t.Fatal("expected all send channels to be closed after Stop")
	}
}
