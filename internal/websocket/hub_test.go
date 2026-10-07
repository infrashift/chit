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
	teams    map[string][]string // userID → team IDs
}

func (f *fakeMembershipChecker) GetChannelIDsForUser(userID string) ([]string, error) {
	return f.channels[userID], nil
}

func (f *fakeMembershipChecker) GetTeamIDsForUser(userID string) ([]string, error) {
	return f.teams[userID], nil
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

func teamEvent(teamID string) *model.WebSocketEvent {
	return &model.WebSocketEvent{
		Event:     model.WebSocketEventChannelCreated,
		Data:      map[string]any{},
		Broadcast: &model.WebSocketBroadcast{TeamID: teamID},
	}
}

func TestHub_TeamEventsOnlyReachTeamMembers(t *testing.T) {
	hub := NewHub(&fakeMembershipChecker{
		teams: map[string][]string{
			"member":     {"team-1"},
			"non-member": {"team-other"},
		},
	})
	defer hub.Stop()

	member := newStubClient(hub, "member")
	nonMember := newStubClient(hub, "non-member")
	hub.Register(member)
	hub.Register(nonMember)
	time.Sleep(50 * time.Millisecond) // allow async membership loads to apply

	hub.Broadcast(teamEvent("team-1"))

	if _, ok := recvWithTimeout(member.send, 100*time.Millisecond); !ok {
		t.Fatal("expected team member to receive event")
	}
	if _, ok := recvWithTimeout(nonMember.send, 50*time.Millisecond); ok {
		t.Fatal("expected non-member NOT to receive team event")
	}
}

func TestHub_TeamMembershipChangeUpdatesFiltering(t *testing.T) {
	hub := NewHub(&fakeMembershipChecker{
		teams: map[string][]string{"user-1": {}},
	})
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(teamEvent("team-1"))
	if _, ok := recvWithTimeout(client.send, 50*time.Millisecond); ok {
		t.Fatal("expected no delivery before joining the team")
	}

	hub.NotifyTeamMembershipChanged("user-1", "team-1", true)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(teamEvent("team-1"))
	if _, ok := recvWithTimeout(client.send, 100*time.Millisecond); !ok {
		t.Fatal("expected delivery after joining the team")
	}

	hub.NotifyTeamMembershipChanged("user-1", "team-1", false)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(teamEvent("team-1"))
	if _, ok := recvWithTimeout(client.send, 50*time.Millisecond); ok {
		t.Fatal("expected no delivery after leaving the team")
	}
}

func TestHub_TeamEventsReachAllWithNilChecker(t *testing.T) {
	hub := NewHub(nil)
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(teamEvent("team-1"))
	if _, ok := recvWithTimeout(client.send, 100*time.Millisecond); !ok {
		t.Fatal("expected delivery on nil-checker hub")
	}
}

func TestHub_BroadcastFromUserRequiresSenderMembership(t *testing.T) {
	hub := NewHub(&fakeMembershipChecker{
		channels: map[string][]string{
			"sender":   {"ch-other"}, // NOT a member of ch-1
			"receiver": {"ch-1"},
		},
	})
	defer hub.Stop()

	sender := newStubClient(hub, "sender")
	receiver := newStubClient(hub, "receiver")
	hub.Register(sender)
	hub.Register(receiver)
	time.Sleep(50 * time.Millisecond)

	typing := &model.WebSocketEvent{
		Event:     model.WebSocketEventTyping,
		Data:      map[string]any{"user_id": "sender"},
		Broadcast: &model.WebSocketBroadcast{ChannelID: "ch-1"},
	}
	hub.BroadcastFromUser("sender", typing)

	if _, ok := recvWithTimeout(receiver.send, 50*time.Millisecond); ok {
		t.Fatal("expected typing event from non-member sender to be dropped")
	}

	// After the sender joins the channel, the same event goes through.
	hub.NotifyMembershipChanged("sender", "ch-1", true)
	time.Sleep(20 * time.Millisecond)

	hub.BroadcastFromUser("sender", typing)
	if _, ok := recvWithTimeout(receiver.send, 100*time.Millisecond); !ok {
		t.Fatal("expected typing event from member sender to be delivered")
	}
}

func TestHub_BroadcastFromUserNilCheckerPassesThrough(t *testing.T) {
	hub := NewHub(nil)
	defer hub.Stop()

	client := newStubClient(hub, "receiver")
	hub.Register(client)
	time.Sleep(20 * time.Millisecond)

	hub.BroadcastFromUser("sender", &model.WebSocketEvent{
		Event:     model.WebSocketEventTyping,
		Data:      map[string]any{},
		Broadcast: &model.WebSocketBroadcast{ChannelID: "ch-1"},
	})

	if _, ok := recvWithTimeout(client.send, 100*time.Millisecond); !ok {
		t.Fatal("expected delivery on nil-checker hub")
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

// fill makes c's send buffer full, so the hub treats it as a slow consumer.
func fill(c *Client) {
	for len(c.send) < cap(c.send) {
		c.send <- &model.WebSocketEvent{Event: "filler"}
	}
}

// One user, three connections, the first and last of them slow. removeClient
// shifted the user's slice in place while broadcastEvent was ranging over it,
// so the loop met the last client twice: the second time after its send
// channel was closed, and a send on a closed channel panics the hub goroutine
// and with it the whole process. The healthy middle client also missed the
// event.
func TestHub_SlowConsumersAmongSeveralConnections(t *testing.T) {
	hub := NewHub(nil)
	defer hub.Stop()

	a, b, c := newStubClient(hub, "user-1"), newStubClient(hub, "user-1"), newStubClient(hub, "user-1")
	for _, cl := range []*Client{a, b, c} {
		hub.Register(cl)
	}
	fill(a)
	fill(c)

	hub.Broadcast(&model.WebSocketEvent{Event: model.WebSocketEventPosted, Broadcast: &model.WebSocketBroadcast{}})

	if ev, ok := recvWithTimeout(b.send, time.Second); !ok || ev.Event != model.WebSocketEventPosted {
		t.Fatal("the healthy connection did not receive the event")
	}
	for name, cl := range map[string]*Client{"first": a, "last": c} {
		closed := false
		for !closed {
			select {
			case _, ok := <-cl.send:
				closed = !ok
			case <-time.After(time.Second):
				t.Fatalf("the %s slow connection was not disconnected", name)
			}
		}
	}
}

// Once the hub has stopped, nothing drains its channels. Register and
// Broadcast used to block forever there, hanging whichever request handler
// called them during shutdown.
func TestHub_CallsAfterStopReturn(t *testing.T) {
	hub := NewHub(nil)
	hub.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		hub.Register(newStubClient(hub, "late"))
		for range sendBufferSize + 1 {
			hub.Broadcast(&model.WebSocketEvent{Event: model.WebSocketEventPosted, Broadcast: &model.WebSocketBroadcast{}})
			hub.BroadcastFromUser("late", channelEvent("c1"))
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a call after Stop blocked")
	}
}

// slowChecker holds membership loads until released, so a test can change
// membership while a load is in flight.
type slowChecker struct {
	fakeMembershipChecker
	release chan struct{}
}

func (s *slowChecker) GetChannelIDsForUser(userID string) ([]string, error) {
	<-s.release
	return s.fakeMembershipChecker.GetChannelIDsForUser(userID)
}

// A membership change that arrived while the user's initial load was in
// flight was dropped ("the load will observe the change"), but the load may
// already have read the database. A user removed from a channel in that
// window kept receiving its events until they reconnected.
func TestHub_MembershipChangeDuringLoadIsKept(t *testing.T) {
	checker := &slowChecker{
		fakeMembershipChecker: fakeMembershipChecker{channels: map[string][]string{"user-1": {"c1"}}},
		release:               make(chan struct{}),
	}
	hub := NewHub(checker)
	defer hub.Stop()

	client := newStubClient(hub, "user-1")
	hub.Register(client)
	hub.NotifyMembershipChanged("user-1", "c1", false) // removed mid-load
	hub.NotifyMembershipChanged("user-1", "c2", true)  // added mid-load
	close(checker.release)
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(channelEvent("c1"))
	hub.Broadcast(channelEvent("c2"))
	ev, ok := recvWithTimeout(client.send, time.Second)
	if !ok || ev.Broadcast.ChannelID != "c2" {
		t.Fatalf("first event = %+v, want only c2's", ev)
	}
	if ev, ok := recvWithTimeout(client.send, 100*time.Millisecond); ok {
		t.Fatalf("received %+v for a channel the user left during the load", ev)
	}
}
