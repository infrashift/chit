package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}

func newTestWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestWSClient_ConnectBadHandshakeIncludesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing authentication header", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	client := ws.NewWSClient(wsURL(srv), "bad-token", 10)
	err := client.Connect()
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should include HTTP status code, got: %v", err)
	}
}

func TestWSClient_ConnectAndReceive(t *testing.T) {
	evt := model.WebSocketEvent{
		Event:    model.WebSocketEventPosted,
		Sequence: 1,
		Data:     map[string]any{"post": "data"},
	}

	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		data, _ := json.Marshal(evt)
		_ = conn.WriteMessage(websocket.TextMessage, data)
		// Keep connection alive briefly.
		time.Sleep(100 * time.Millisecond)
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", 10)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	select {
	case got := <-client.Events():
		if got.Event != model.WebSocketEventPosted {
			t.Errorf("event = %q, want %q", got.Event, model.WebSocketEventPosted)
		}
		if got.Sequence != 1 {
			t.Errorf("sequence = %d, want 1", got.Sequence)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestWSClient_Send(t *testing.T) {
	received := make(chan model.WebSocketMessage, 1)

	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var wsMsg model.WebSocketMessage
		if err := json.Unmarshal(msg, &wsMsg); err == nil {
			received <- wsMsg
		}
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", 10)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	msg := model.WebSocketMessage{Action: "typing", Seq: 1, Data: map[string]any{"channel_id": "c1"}}
	if err := client.Send(msg); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-received:
		if got.Action != "typing" {
			t.Errorf("action = %q, want %q", got.Action, "typing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestWSClient_Close(t *testing.T) {
	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		// Echo server — keep reading until closed.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", 10)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestBackoff_ExponentialIncrease(t *testing.T) {
	b := ws.NewBackoff()
	d1 := b.Next()
	d2 := b.Next()
	d3 := b.Next()

	if d2 <= d1 {
		t.Errorf("expected d2 (%v) > d1 (%v)", d2, d1)
	}
	if d3 <= d2 {
		t.Errorf("expected d3 (%v) > d2 (%v)", d3, d2)
	}
}

func TestBackoff_CapsAtMax(t *testing.T) {
	b := ws.NewBackoff()
	var last time.Duration
	for range 100 {
		last = b.Next()
	}
	if last > b.Max {
		t.Errorf("backoff %v exceeds max %v", last, b.Max)
	}
}

func TestBackoff_Reset(t *testing.T) {
	b := ws.NewBackoff()
	_ = b.Next()
	_ = b.Next()
	_ = b.Next()
	b.Reset()
	d := b.Next()
	if d != b.Base {
		t.Errorf("after reset, first backoff = %v, want %v", d, b.Base)
	}
}

func TestWSClient_ReconnectsAfterDrop(t *testing.T) {
	connections := make(chan *websocket.Conn, 5)
	evt := model.WebSocketEvent{
		Event:    model.WebSocketEventPosted,
		Sequence: 1,
		Data:     map[string]any{"post": "reconnected"},
	}

	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		connections <- conn
		// Send one event then close (simulating server drop)
		data, _ := json.Marshal(evt)
		_ = conn.WriteMessage(websocket.TextMessage, data)
		// Wait a moment then close to trigger reconnect
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close()
		// The handler returns but the server stays up,
		// so the client can reconnect to a new handler invocation.
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", 10)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	// Wait for first connection
	select {
	case <-connections:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first connection")
	}

	// Receive event from first connection
	select {
	case got := <-client.Events():
		if got.Event != model.WebSocketEventPosted {
			t.Errorf("event = %q, want %q", got.Event, model.WebSocketEventPosted)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Wait for reconnection (second connection)
	select {
	case <-connections:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for reconnect")
	}

	// Receive event from reconnected connection
	select {
	case got := <-client.Events():
		if got.Event != model.WebSocketEventPosted {
			t.Errorf("reconnected event = %q, want %q", got.Event, model.WebSocketEventPosted)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event after reconnect")
	}
}

func TestWSClient_CloseStopsReconnect(t *testing.T) {
	connected := make(chan struct{}, 1)
	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		connected <- struct{}{}
		// Close immediately to trigger reconnect attempt
		_ = conn.Close()
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", 10)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}

	// Wait for first connection
	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connection")
	}

	// Close client — should stop the reconnect loop
	if err := client.Close(); err != nil {
		t.Logf("Close() error (expected for already-closed conn): %v", err)
	}

	// Give it time to verify no panic / hang
	time.Sleep(200 * time.Millisecond)
}

// A drop has to be observable. Before this, reconnect was entirely internal
// and the UI had no way to know the socket had died.
func TestConnStateReportsConnectAndDrop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Accept, then hang up, which is what a server restart looks like.
		_ = conn.Close()
	}))
	defer srv.Close()

	c := ws.NewWSClient("ws"+strings.TrimPrefix(srv.URL, "http"), "token", 8)
	defer func() { _ = c.Close() }()

	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// First transition: connected.
	select {
	case st := <-c.State():
		if !st.Connected {
			t.Fatalf("first state = %+v, want connected", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no connect state reported")
	}

	// Second: the drop.
	select {
	case st := <-c.State():
		if st.Connected {
			t.Errorf("second state = %+v, want disconnected", st)
		}
		if st.Err == nil {
			t.Error("disconnect carried no error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no disconnect state reported")
	}
}

// Retrying cannot fix rejected credentials, so the client must say so rather
// than dialing forever behind a dead session.
func TestConnStateReportsUnauthorized(t *testing.T) {
	var upgraded bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !upgraded {
			upgraded = true
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			_ = conn.Close()
			return
		}
		// The reconnect attempt is rejected.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := ws.NewWSClient("ws"+strings.TrimPrefix(srv.URL, "http"), "token", 8)
	defer func() { _ = c.Close() }()

	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case st := <-c.State():
			if st.Unauthorized {
				return // what we are waiting for
			}
		case <-deadline:
			t.Fatal("no unauthorized state reported")
		}
	}
}

// A full event buffer used to discard events in silence. The socket stays up,
// so no disconnect is reported and the caller has no way to learn its view has
// stopped matching the server. The drop itself is unavoidable — going unheard
// is not.
func TestWSClient_ReportsDesyncWhenTheBufferOverflows(t *testing.T) {
	const buffered = 2

	srv := newTestWSServer(t, func(conn *websocket.Conn) {
		// Comfortably more than the buffer holds, so the reader — which never
		// drains here — must overflow.
		for i := range buffered * 10 {
			data, _ := json.Marshal(model.WebSocketEvent{
				Event:    model.WebSocketEventPosted,
				Sequence: int64(i),
			})
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	})

	client := ws.NewWSClient(wsURL(srv), "test-token", buffered)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case st := <-client.State():
			if st.Desynced {
				if !st.Connected {
					t.Error("desync reported as a disconnect; the socket is still up")
				}
				return
			}
		case <-deadline:
			t.Fatal("events were dropped without reporting a desync")
		}
	}
}
