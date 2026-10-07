package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/model"
)

// readEvents collects WebSocket events until want has been seen or the
// deadline passes.
func readEvents(t *testing.T, conn *gws.Conn, want string) []model.WebSocketEvent {
	t.Helper()
	var got []model.WebSocketEvent
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var evt model.WebSocketEvent
		if err := conn.ReadJSON(&evt); err != nil {
			return got
		}
		got = append(got, evt)
		if evt.Event == want {
			return got
		}
	}
}

// A reply changes its thread's counters, and clients learn the new count
// from thread_updated. Nothing sent it, so reply counts did not update live.
func TestCreateReplyBroadcastsThreadUpdated(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	srv := httptest.NewServer(New(a))
	defer srv.Close()

	header := http.Header{"X-User-Id": {testKratosID}}
	conn, resp, err := gws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/websocket", header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(50 * time.Millisecond) // let the hub register the client

	body := `{"channel_id":"` + testChannelID + `","root_id":"` + testRootPost + `","content":"a reply"}`
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/v1/posts", strings.NewReader(body))
	req.Header = header.Clone()
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create reply: status %d", res.StatusCode)
	}

	events := readEvents(t, conn, model.WebSocketEventThreadUpdated)
	if len(events) == 0 || events[len(events)-1].Event != model.WebSocketEventThreadUpdated {
		t.Fatalf("no thread_updated; got %+v", events)
	}
	thread, _ := events[len(events)-1].Data["thread"].(map[string]any)
	if thread["post_id"] != testRootPost {
		t.Errorf("thread_updated for %v, want the root %s", thread["post_id"], testRootPost)
	}
	if n, _ := thread["reply_count"].(float64); n < 1 {
		t.Errorf("reply_count = %v, want the new reply counted", thread["reply_count"])
	}
	if _, has := events[len(events)-1].Data["post"]; has {
		t.Error("thread_updated carries the reply; clients would show it twice")
	}
}
