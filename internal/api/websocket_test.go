package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/model"
	ws "github.com/infrashift/chit/internal/websocket"
)

// serveWS runs handleWebSocket behind a test server that authenticates every
// request as user (or nobody, when user is nil).
func serveWS(t *testing.T, user *model.User) (url string, publish func(*model.WebSocketEvent)) {
	t.Helper()
	a, _, cleanup := setupTestApp(t)
	t.Cleanup(cleanup)
	h := handleWebSocket(a)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user != nil {
			r = authedRequest(r, user)
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), a.Hub.Broadcast
}

func TestWebSocket_DeliversEventsToTheConnectedUser(t *testing.T) {
	url, publish := serveWS(t, testUser())
	conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = resp.Body.Close()

	// Registration completes before the handler returns, so the event cannot
	// race ahead of it.
	publish(&model.WebSocketEvent{
		Event:     model.WebSocketEventPosted,
		Data:      map[string]any{"n": 1},
		Broadcast: &model.WebSocketBroadcast{UserID: testUserID},
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ev model.WebSocketEvent
	if err := conn.ReadJSON(&ev); err != nil {
		t.Fatalf("read: %v", err)
	}
	if ev.Event != model.WebSocketEventPosted {
		t.Fatalf("got %q, want posted", ev.Event)
	}
}

func TestWebSocket_RejectsAnUnauthenticatedCaller(t *testing.T) {
	url, _ := serveWS(t, nil)
	_, resp, err := websocket.DefaultDialer.DialContext(t.Context(), url, nil)
	if err == nil {
		t.Fatal("an unauthenticated caller connected")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("got %d %q, want a JSON 401", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

// Typing is the one message a client may send; it is rebroadcast to the
// channel with the sender's id.
func TestWebSocket_TypingIsRebroadcast(t *testing.T) {
	url, _ := serveWS(t, testUser())
	dial := func() *websocket.Conn {
		t.Helper()
		conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		_ = resp.Body.Close()
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	sender, watcher := dial(), dial()

	if err := sender.WriteJSON(map[string]any{
		"action": "typing", "data": map[string]any{"channel_id": testChannelID},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = watcher.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ev model.WebSocketEvent
	if err := watcher.ReadJSON(&ev); err != nil {
		t.Fatalf("read: %v", err)
	}
	if ev.Event != model.WebSocketEventTyping || ev.Data["user_id"] != testUserID || ev.Broadcast.ChannelID != testChannelID {
		t.Fatalf("got %+v, want a typing event from the sender in the channel", ev)
	}
}

// storeMembership is the hub's membership checker over the test store, as
// server.hubMembershipAdapter is over the SQL store, so channel events are
// filtered by membership the way they are in production.
type storeMembership struct{ ms *mockStore }

func (s storeMembership) GetChannelIDsForUser(userID string) ([]string, error) {
	return s.ms.Channels.GetChannelIDsForUser(context.Background(), userID)
}

func (s storeMembership) GetTeamIDsForUser(userID string) ([]string, error) {
	teams, err := s.ms.Teams.GetTeamsForUser(context.Background(), userID)
	ids := make([]string, 0, len(teams))
	for _, tm := range teams {
		ids = append(ids, tm.ID)
	}
	return ids, err
}

// A user removed from a channel never got user_removed: the hub drops them
// from the channel's audience before it delivers the channel broadcast.
func TestWebSocket_RemovedUserIsTold(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()
	a.Hub = ws.NewHub(storeMembership{ms})
	defer a.Hub.Stop()

	h := handleWebSocket(a)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, authedRequest(r, testUser()))
	}))
	defer srv.Close()
	conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = resp.Body.Close()

	read := func() model.WebSocketEvent {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var ev model.WebSocketEvent
		if err := conn.ReadJSON(&ev); err != nil {
			t.Fatalf("read: %v", err)
		}
		return ev
	}
	// Wait until the hub has loaded the user's memberships: channel events
	// are dropped until it has. Probes keep coming until one arrives; a read
	// timeout would be permanent on a gorilla conn, so the read just blocks.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				a.Hub.Broadcast(&model.WebSocketEvent{Event: "probe", Broadcast: &model.WebSocketBroadcast{ChannelID: testChannelID}})
			}
		}
	}()
	if ev := read(); ev.Event != "probe" {
		t.Fatalf("got %+v before any probe", ev)
	}
	close(stop)

	// Leaving is removal too, and needs no admin.
	if err := a.RemoveChannelMember(t.Context(), testChannelID, testUserID, testUserID); err != nil {
		t.Fatalf("RemoveChannelMember: %v", err)
	}
	// Probes already queued may still arrive; skip them.
	ev := read()
	for ev.Event == "probe" {
		ev = read()
	}
	if ev.Event != model.WebSocketEventUserRemoved || ev.Data["user_id"] != testUserID || ev.Data["channel_id"] != testChannelID {
		t.Fatalf("got %+v, want user_removed for the removed user", ev)
	}
}
