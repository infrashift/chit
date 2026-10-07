package api

import (
	"net/http"
	"net/http/httptest"
	"slices"
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

// dialAs connects a WebSocket to the full router as the test user.
func dialAs(t *testing.T, srv *httptest.Server) *gws.Conn {
	t.Helper()
	conn, resp, err := gws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/websocket",
		http.Header{"X-User-Id": {testKratosID}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.Close() })
	time.Sleep(50 * time.Millisecond) // let the hub register the client
	return conn
}

func send(t *testing.T, method, url, body string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	req.Header.Set("X-User-Id", testKratosID)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d", method, url, res.StatusCode)
	}
}

func tagNames(t *testing.T, evt model.WebSocketEvent) []string {
	t.Helper()
	raw, _ := evt.Data["tags"].([]any)
	names := []string{}
	for _, r := range raw {
		tag, _ := r.(map[string]any)
		name, _ := tag["name"].(string)
		names = append(names, name)
	}
	return names
}

// Tags are added after a post is created, by a separate request. Nothing
// announced them, so other clients showed a post's tags only after
// reloading the channel. Each change now goes to the post's channel with
// the post's whole tag list.
func TestTaggingAPostBroadcastsItsTags(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()
	ms.Tags.Seed(&model.Tag{ID: "019421a0-0000-7000-8000-000000000071", Name: "deploy"})
	srv := httptest.NewServer(New(a))
	defer srv.Close()
	conn := dialAs(t, srv)
	postTags := srv.URL + "/api/v1/posts/" + testRootPost + "/tags"

	send(t, http.MethodPost, postTags, `{"tag_id":"019421a0-0000-7000-8000-000000000071"}`)
	events := readEvents(t, conn, model.WebSocketEventPostTagsUpdated)
	if len(events) == 0 || events[len(events)-1].Event != model.WebSocketEventPostTagsUpdated {
		t.Fatalf("no post_tags_updated after tagging; got %+v", events)
	}
	added := events[len(events)-1]
	if added.Data["post_id"] != testRootPost || added.Data["channel_id"] != testChannelID {
		t.Errorf("event names post %v in %v", added.Data["post_id"], added.Data["channel_id"])
	}
	if names := tagNames(t, added); !slices.Contains(names, "deploy") || !slices.Contains(names, "important") {
		t.Errorf("tags = %v, want the whole list, important and deploy", names)
	}

	send(t, http.MethodDelete, postTags+"/019421a0-0000-7000-8000-000000000071", "")
	events = readEvents(t, conn, model.WebSocketEventPostTagsUpdated)
	if len(events) == 0 {
		t.Fatal("no post_tags_updated after untagging")
	}
	if names := tagNames(t, events[len(events)-1]); slices.Contains(names, "deploy") {
		t.Errorf("tags = %v after removing deploy", names)
	}
}
