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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)
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
	hub := NewHub(nil)

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

// ─── Membership filtering ────────────────────────────────────────

type fakeMembershipChecker struct {
	channels map[string][]string // userID → channel IDs
}

func (f *fakeMembershipChecker) GetChannelIDsForUser(userID string) ([]string, error) {
	return f.channels[userID], nil
}

func channelEvent(channelID string) *model.WebSocketEvent {
	return &model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{},
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	}
}

func TestHub_ChannelEventsOnlyReachMembers(t *testing.T) {
	hub := NewHub(&fakeMembershipChecker{
		channels: map[string][]string{
			"member":     {"ch-1"},
			"non-member": {"ch-other"},
		},
	})
	defer hub.Stop()

	member := newStubClient(hub, "member")
	nonMember := newStubClient(hub, "non-member")
	hub.Register(member)
	hub.Register(nonMember)
	time.Sleep(50 * time.Millisecond) // allow async membership loads to apply

	hub.Broadcast(channelEvent("ch-1"))

	if _, ok := recvWithTimeout(member.send, 100*time.Millisecond); !ok {
		t.Fatal("expected channel member to receive event")
	}
	if _, ok := recvWithTimeout(nonMember.send, 50*time.Millisecond); ok {
		t.Fatal("expected non-member NOT to receive channel event")
	}
}

func TestHub_MembershipChangeUpdatesFiltering(t *testing.T) {
	hub := NewHub(&fakeMembershipChecker{
		channels: map[string][]string{"user-1": {}},
	})
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(channelEvent("ch-1"))
	if _, ok := recvWithTimeout(client.send, 50*time.Millisecond); ok {
		t.Fatal("expected no delivery before joining the channel")
	}

	hub.NotifyMembershipChanged("user-1", "ch-1", true)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(channelEvent("ch-1"))
	if _, ok := recvWithTimeout(client.send, 100*time.Millisecond); !ok {
		t.Fatal("expected delivery after joining the channel")
	}

	hub.NotifyMembershipChanged("user-1", "ch-1", false)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(channelEvent("ch-1"))
	if _, ok := recvWithTimeout(client.send, 50*time.Millisecond); ok {
		t.Fatal("expected no delivery after leaving the channel")
	}
}

// Regression test: unregistering the same client twice (slow-consumer
// disconnect racing the readPump's deferred Unregister) must not panic with
// "close of closed channel".
func TestHub_DoubleUnregisterNoPanic(t *testing.T) {
	hub := NewHub(nil)
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	hub.Unregister(client)
	hub.Unregister(client)
	time.Sleep(20 * time.Millisecond)

	if _, ok := <-client.send; ok {
		t.Fatal("expected send channel to be closed")
	}
}

// The slow-consumer disconnect is now synchronous inside the event loop, so
// the send channel MUST be closed once the overflowing broadcast is processed.
func TestHub_SlowConsumerIsDisconnected(t *testing.T) {
	hub := NewHub(nil)
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < sendBufferSize+1; i++ {
		hub.Broadcast(&model.WebSocketEvent{
			Event:     model.WebSocketEventPosted,
			Data:      map[string]any{"i": i},
			Broadcast: &model.WebSocketBroadcast{},
		})
	}

	// Wait for the hub to process every broadcast before draining, so the
	// overflowing event deterministically finds the buffer full.
	time.Sleep(100 * time.Millisecond)

	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-client.send:
			if !ok {
				// A deferred readPump-style unregister must also be harmless.
				hub.Unregister(client)
				time.Sleep(20 * time.Millisecond)
				return
			}
		case <-timeout:
			t.Fatal("expected slow consumer to be disconnected (send channel closed)")
		}
	}
}
