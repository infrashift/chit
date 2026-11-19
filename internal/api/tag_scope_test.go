package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/model"
)

// Scoped Tags (P-CHIT-7). The privacy tests are the point: a private Tag
// must never reach anybody but its owner, by any path. Each path has its own
// negative test below. "alice" (testUser) and "bob" (extraUser) are both
// members of the test channel; bob owns the private Tags alice must not see.

const (
	aliceKratos = testKratosID
	bobKratos   = "kratos-002"
)

func newTagEnv(t *testing.T) *bridgeEnv {
	t.Helper()
	e := newBridgeEnv(t)
	e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: testChannelID, UserID: extraUserID, CreateAt: 1000})
	return e
}

// call sends a request as who ("user:<kratos>" or "client:<id>") and returns
// the status and body.
func (e *bridgeEnv) call(t *testing.T, method, path, who, body string, headers map[string]string) (code int, respBody string) {
	t.Helper()
	w := e.do(t, method, path, who, body, headers)
	return w.Code, w.Body.String()
}

func (e *bridgeEnv) mustCreateTag(t *testing.T, who, name, scope, channelID string, headers map[string]string) *model.Tag {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"scope":%q`, name, scope)
	if channelID != "" {
		body += fmt.Sprintf(`,"channel_id":%q`, channelID)
	}
	code, out := e.call(t, http.MethodPost, "/api/v1/tags", who, body+"}", headers)
	if code != http.StatusCreated {
		t.Fatalf("create tag %q as %s: %d %s", name, who, code, out)
	}
	var tag model.Tag
	if err := json.Unmarshal([]byte(out), &tag); err != nil {
		t.Fatal(err)
	}
	return &tag
}

func (e *bridgeEnv) mustOK(t *testing.T, method, path, who, body string) {
	t.Helper()
	if code, out := e.call(t, method, path, who, body, nil); code >= 300 {
		t.Fatalf("%s %s as %s: %d %s", method, path, who, code, out)
	}
}

func tagIDsIn(t *testing.T, body string) []string {
	t.Helper()
	var tags []model.Tag
	if err := json.Unmarshal([]byte(body), &tags); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	ids := make([]string, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
	}
	return ids
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// bobsPrivateTagOnEverything has bob create a private Tag and apply it to the
// root post and to the channel; it also makes one channel Tag for contrast.
func bobsPrivateTagOnEverything(t *testing.T, e *bridgeEnv) (private, channel *model.Tag) {
	t.Helper()
	private = e.mustCreateTag(t, "user:"+bobKratos, "bobs secret", model.TagScopePrivate, "", nil)
	channel = e.mustCreateTag(t, "user:"+bobKratos, "shared", model.TagScopeChannel, testChannelID, nil)
	for _, tag := range []*model.Tag{private, channel} {
		e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", "user:"+bobKratos, `{"tag_id":"`+tag.ID+`"}`)
		e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", "user:"+bobKratos, `{"tag_id":"`+tag.ID+`"}`)
	}
	return private, channel
}

// ─── Privacy: one negative test per path ─────────────────────────

func TestTagPrivacy_ListShowsNoOtherUsersPrivateTag(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)
	for _, path := range []string{"/api/v1/tags", "/api/v1/tags?channel_id=" + testChannelID} {
		_, body := e.call(t, http.MethodGet, path, "user:"+aliceKratos, "", nil)
		ids := tagIDsIn(t, body)
		if contains(ids, private.ID) {
			t.Errorf("%s: alice sees bob's private tag", path)
		}
		if !contains(ids, channel.ID) {
			t.Errorf("%s: alice does not see the channel tag", path)
		}
		_, body = e.call(t, http.MethodGet, path, "user:"+bobKratos, "", nil)
		if !contains(tagIDsIn(t, body), private.ID) {
			t.Errorf("%s: bob does not see his own private tag", path)
		}
	}
}

func TestTagPrivacy_PostReadsShowNoOtherUsersPrivateTag(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)

	_, body := e.call(t, http.MethodGet, "/api/v1/posts/"+testRootPost+"/tags", "user:"+aliceKratos, "", nil)
	if ids := tagIDsIn(t, body); contains(ids, private.ID) || !contains(ids, channel.ID) {
		t.Errorf("per-post read for alice = %v", ids)
	}

	_, body = e.call(t, http.MethodPost, "/api/v1/tags/posts", "user:"+aliceKratos, `{"post_ids":["`+testRootPost+`"]}`, nil)
	var batch map[string][]model.Tag
	if err := json.Unmarshal([]byte(body), &batch); err != nil {
		t.Fatal(err)
	}
	for _, tag := range batch[testRootPost] {
		if tag.ID == private.ID {
			t.Error("batch read: alice sees bob's private tag")
		}
	}
	if len(batch[testRootPost]) != 2 { // "important" (seeded) and "shared"
		t.Errorf("batch read for alice = %v, want the two channel tags", batch[testRootPost])
	}
}

func TestTagPrivacy_ChannelReadShowsNoOtherUsersPrivateTag(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)
	_, body := e.call(t, http.MethodGet, "/api/v1/channels/"+testChannelID+"/tags", "user:"+aliceKratos, "", nil)
	if ids := tagIDsIn(t, body); contains(ids, private.ID) || !contains(ids, channel.ID) {
		t.Errorf("channel read for alice = %v", ids)
	}
}

// Searching by somebody else's private Tag must find nothing, exactly as an
// unknown Tag does, even though the tagged post is in alice's channel.
func TestTagPrivacy_SearchByOtherUsersPrivateTagFindsNothing(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)
	search := func(who, tagID string) int {
		_, body := e.call(t, http.MethodPost, "/api/v1/posts/search", who, `{"tag_ids":["`+tagID+`"]}`, nil)
		var list model.PostList
		if err := json.Unmarshal([]byte(body), &list); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
		return len(list.Order)
	}
	if n := search("user:"+aliceKratos, private.ID); n != 0 {
		t.Errorf("alice searching by bob's private tag found %d posts", n)
	}
	if n := search("user:"+aliceKratos, model.NewID()); n != 0 {
		t.Errorf("an unknown tag found %d posts", n)
	}
	if n := search("user:"+bobKratos, private.ID); n != 1 {
		t.Errorf("bob searching by his own tag found %d posts, want 1", n)
	}
	if n := search("user:"+aliceKratos, channel.ID); n != 1 {
		t.Errorf("alice searching by the channel tag found %d posts, want 1", n)
	}
}

func TestTagPrivacy_ChannelFilter(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)
	filter := func(who string, tagIDs ...string) []model.TaggedChannel {
		code, body := e.call(t, http.MethodPost, "/api/v1/tags/channels", who, `{"tag_ids":["`+strings.Join(tagIDs, `","`)+`"]}`, nil)
		if code != http.StatusOK {
			t.Fatalf("filter: %d %s", code, body)
		}
		var out []model.TaggedChannel
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := filter("user:"+aliceKratos, private.ID); len(got) != 0 {
		t.Errorf("alice filtering by bob's private tag got %d channels", len(got))
	}
	got := filter("user:"+aliceKratos, channel.ID)
	if len(got) != 1 || got[0].Channel.ID != testChannelID {
		t.Fatalf("alice filtering by the channel tag got %+v", got)
	}
	for _, tag := range got[0].Tags {
		if tag.ID == private.ID {
			t.Error("the filter result shows alice bob's private tag")
		}
	}
	if got := filter("user:"+bobKratos, private.ID, channel.ID); len(got) != 1 {
		t.Errorf("bob filtering by both tags got %d channels, want 1", len(got))
	}
}

// Every operation that names somebody else's private Tag by ID is a 404, the
// same answer as for a Tag that does not exist.
func TestTagPrivacy_DirectAccessToOtherUsersPrivateTagIs404(t *testing.T) {
	e := newTagEnv(t)
	private, _ := bobsPrivateTagOnEverything(t, e)
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/posts/" + testReplyPost + "/tags", `{"tag_id":"` + private.ID + `"}`},
		{http.MethodDelete, "/api/v1/posts/" + testRootPost + "/tags/" + private.ID, ""},
		{http.MethodPost, "/api/v1/channels/" + testChannelID + "/tags", `{"tag_id":"` + private.ID + `"}`},
		{http.MethodDelete, "/api/v1/channels/" + testChannelID + "/tags/" + private.ID, ""},
		{http.MethodPut, "/api/v1/tags/" + private.ID, `{"name":"renamed"}`},
		{http.MethodDelete, "/api/v1/tags/" + private.ID, ""},
	}
	for _, c := range cases {
		if code, body := e.call(t, c.method, c.path, "user:"+aliceKratos, c.body, nil); code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want 404: %s", c.method, c.path, code, body)
		}
	}
	// And bob's tag is still on the root post.
	_, body := e.call(t, http.MethodGet, "/api/v1/posts/"+testRootPost+"/tags", "user:"+bobKratos, "", nil)
	if !contains(tagIDsIn(t, body), private.ID) {
		t.Error("alice removed bob's private tag")
	}
}

// wsAs connects a WebSocket to srv as the person with this Kratos ID.
func wsAs(t *testing.T, srv *httptest.Server, kratosID string) *gws.Conn {
	t.Helper()
	conn, resp, err := gws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/websocket",
		http.Header{"X-User-Id": {kratosID}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close handshake response body: %v", err)
		}
	}
	t.Cleanup(func() { closeOK(t, conn, "websocket") })
	return conn
}

// drainUntil reads events until one named sentinel arrives, and returns the
// events before it.
func drainUntil(t *testing.T, conn *gws.Conn, sentinel string) []model.WebSocketEvent {
	t.Helper()
	var got []model.WebSocketEvent
	readDeadlineOK(t, conn, 3*time.Second)
	for {
		var evt model.WebSocketEvent
		if err := conn.ReadJSON(&evt); err != nil {
			t.Fatalf("no sentinel %q: %v (after %v)", sentinel, err, got)
		}
		if evt.Event == sentinel {
			return got
		}
		got = append(got, evt)
	}
}

// Every change to a private Tag (apply, remove, on a post or a channel,
// rename, delete) reaches the owner's sessions and nobody else's.
func TestTagPrivacy_WebSocketEventsReachOnlyTheOwner(t *testing.T) {
	e := newTagEnv(t)
	srv := httptest.NewServer(e.router)
	defer srv.Close()
	alice, bob := wsAs(t, srv, aliceKratos), wsAs(t, srv, bobKratos)
	time.Sleep(100 * time.Millisecond) // let the hub register both

	private := e.mustCreateTag(t, "user:"+bobKratos, "bobs secret", model.TagScopePrivate, "", nil)
	bobAs := "user:" + bobKratos
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", bobAs, `{"tag_id":"`+private.ID+`"}`)
	e.mustOK(t, http.MethodDelete, "/api/v1/posts/"+testRootPost+"/tags/"+private.ID, bobAs, "")
	e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", bobAs, `{"tag_id":"`+private.ID+`"}`)
	e.mustOK(t, http.MethodPut, "/api/v1/tags/"+private.ID, bobAs, `{"name":"bobs other secret"}`)
	e.mustOK(t, http.MethodDelete, "/api/v1/tags/"+private.ID, bobAs, "")

	// A sentinel to each, queued after every tag event above: the hub
	// delivers in order, so anything alice was sent arrives before it.
	for _, id := range []string{testUserID, extraUserID} {
		e.a.Hub.Broadcast(&model.WebSocketEvent{Event: "sentinel", Broadcast: &model.WebSocketBroadcast{UserID: id}})
	}
	if leaked := drainUntil(t, alice, "sentinel"); len(leaked) != 0 {
		t.Errorf("alice received %d events about bob's private tag: %+v", len(leaked), leaked)
	}
	got := map[string]int{}
	for _, ev := range drainUntil(t, bob, "sentinel") {
		got[ev.Event]++
	}
	want := map[string]int{
		model.WebSocketEventPostTagsUpdated: 2, model.WebSocketEventChannelTagsUpdated: 1,
		model.WebSocketEventTagUpdated: 1, model.WebSocketEventTagDeleted: 1,
	}
	for ev, n := range want {
		if got[ev] != n {
			t.Errorf("bob got %d %s events, want %d (all: %v)", got[ev], ev, n, got)
		}
	}
}

// A channel Tag's change reaches every member, each with their own view:
// alice's event about the post never lists bob's private Tag on it.
func TestTagPrivacy_ChannelTagEventCarriesEachReadersView(t *testing.T) {
	e := newTagEnv(t)
	srv := httptest.NewServer(e.router)
	defer srv.Close()
	private := e.mustCreateTag(t, "user:"+bobKratos, "bobs secret", model.TagScopePrivate, "", nil)
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", "user:"+bobKratos, `{"tag_id":"`+private.ID+`"}`)
	alice := wsAs(t, srv, aliceKratos)
	time.Sleep(100 * time.Millisecond)

	shared := e.mustCreateTag(t, "user:"+aliceKratos, "shared", model.TagScopeChannel, testChannelID, nil)
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", "user:"+aliceKratos, `{"tag_id":"`+shared.ID+`"}`)
	e.a.Hub.Broadcast(&model.WebSocketEvent{Event: "sentinel", Broadcast: &model.WebSocketBroadcast{UserID: testUserID}})
	events := drainUntil(t, alice, "sentinel")
	if len(events) != 1 {
		t.Fatalf("alice got %d events, want 1: %+v", len(events), events)
	}
	tags, ok := events[0].Data["tags"].([]any)
	if !ok {
		t.Fatalf("alice's event has no tag list: %+v", events[0].Data)
	}
	for _, raw := range tags {
		tag, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tag %v is not an object", raw)
		}
		if tag["id"] == private.ID {
			t.Error("alice's post_tags_updated lists bob's private tag")
		}
	}
}

// recordingPubSub records what is published to the events topic, the
// out-of-process feed: the nearest thing Chit has to an export.
type recordingPubSub struct {
	mu   sync.Mutex
	msgs []string
}

func (p *recordingPubSub) Publish(_ context.Context, _ string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, string(data))
	return nil
}
func (p *recordingPubSub) Subscribe(_ context.Context, _ string, _ func([]byte)) error { return nil }
func (p *recordingPubSub) Close() error                                                { return nil }

func TestTagPrivacy_PrivateTagChangesAreNotPublished(t *testing.T) {
	e := newTagEnv(t)
	rec := &recordingPubSub{}
	e.a.PubSub = rec
	private := e.mustCreateTag(t, "user:"+bobKratos, "bobs secret", model.TagScopePrivate, "", nil)
	bobAs := "user:" + bobKratos
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", bobAs, `{"tag_id":"`+private.ID+`"}`)
	e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", bobAs, `{"tag_id":"`+private.ID+`"}`)
	e.mustOK(t, http.MethodPut, "/api/v1/tags/"+private.ID, bobAs, `{"name":"renamed"}`)
	e.mustOK(t, http.MethodDelete, "/api/v1/tags/"+private.ID, bobAs, "")
	if len(rec.msgs) != 0 {
		t.Errorf("private tag changes were published: %v", rec.msgs)
	}

	shared := e.mustCreateTag(t, bobAs, "shared", model.TagScopeChannel, testChannelID, nil)
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", bobAs, `{"tag_id":"`+shared.ID+`"}`)
	if len(rec.msgs) != 1 {
		t.Errorf("a channel tag change published %d envelopes, want 1", len(rec.msgs))
	}
}

// ─── Deletion ────────────────────────────────────────────────────

func TestDeleteUserTags(t *testing.T) {
	e := newTagEnv(t)
	private, channel := bobsPrivateTagOnEverything(t, e)
	// alice applies bob's channel tag to the reply: her application stays,
	// his tag does not.
	aliceTag := e.mustCreateTag(t, "user:"+aliceKratos, "alices", model.TagScopeChannel, testChannelID, nil)
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testReplyPost+"/tags", "user:"+bobKratos, `{"tag_id":"`+aliceTag.ID+`"}`)

	if code, _ := e.call(t, http.MethodDelete, "/api/v1/users/"+extraUserID+"/tags", "user:"+aliceKratos, "", nil); code != http.StatusForbidden {
		t.Fatalf("alice deleting bob's tags: got %d, want 403", code)
	}
	e.mustOK(t, http.MethodDelete, "/api/v1/users/"+extraUserID+"/tags", "user:"+bobKratos, "")

	for _, id := range []string{private.ID, channel.ID} {
		if _, err := e.ms.Tags.Get(t.Context(), id); err == nil {
			t.Errorf("tag %s survived its owner's deletion", id)
		}
		if _, applied := e.ms.Tags.AppliedBy(testRootPost, id); applied {
			t.Errorf("tag %s is still applied", id)
		}
	}
	by, applied := e.ms.Tags.AppliedBy(testReplyPost, aliceTag.ID)
	if !applied || by != "" {
		t.Errorf("alice's tag that bob applied: applied=%v by=%q, want applied and anonymous", applied, by)
	}
}

// ─── Scope rules ─────────────────────────────────────────────────

func TestTags_NamesAreUniquePerScopeAfterNFKCAndLowercasing(t *testing.T) {
	e := newTagEnv(t)
	a1 := e.mustCreateTag(t, "user:"+aliceKratos, "Bug Fix", model.TagScopePrivate, "", nil)
	a2 := e.mustCreateTag(t, "user:"+aliceKratos, "ＢＵＧ fix", model.TagScopePrivate, "", nil)
	if a1.ID != a2.ID {
		t.Error("one owner got two private tags with one key")
	}
	b1 := e.mustCreateTag(t, "user:"+bobKratos, "bug fix", model.TagScopePrivate, "", nil)
	if b1.ID == a1.ID {
		t.Error("two owners share a private tag")
	}
	c1 := e.mustCreateTag(t, "user:"+aliceKratos, "bug fix", model.TagScopeChannel, testChannelID, nil)
	c2 := e.mustCreateTag(t, "user:"+bobKratos, "BUG FIX", model.TagScopeChannel, testChannelID, nil)
	if c1.ID != c2.ID || c1.OwnerID != testUserID {
		t.Error("one channel got two channel tags with one key")
	}
	if code, _ := e.call(t, http.MethodPut, "/api/v1/tags/"+a1.ID, "user:"+aliceKratos, `{"name":"other"}`, nil); code != http.StatusOK {
		t.Fatalf("rename: %d", code)
	}
	e.mustCreateTag(t, "user:"+aliceKratos, "taken", model.TagScopePrivate, "", nil)
	if code, _ := e.call(t, http.MethodPut, "/api/v1/tags/"+a1.ID, "user:"+aliceKratos, `{"name":"TAKEN"}`, nil); code != http.StatusConflict {
		t.Errorf("rename onto a taken key: got %d, want 409", code)
	}
}

func TestTags_ScopeRules(t *testing.T) {
	e := newTagEnv(t)
	shared := e.mustCreateTag(t, "user:"+aliceKratos, "shared", model.TagScopeChannel, testChannelID, nil)
	cases := []struct {
		name, method, path, who, body string
		want                          int
	}{
		{"a member applies another member's channel tag", http.MethodPost, "/api/v1/posts/" + testRootPost + "/tags", "user:" + bobKratos, `{"tag_id":"` + shared.ID + `"}`, http.StatusOK},
		{"only the owner renames a channel tag", http.MethodPut, "/api/v1/tags/" + shared.ID, "user:" + bobKratos, `{"name":"mine now"}`, http.StatusForbidden},
		{"only the owner deletes a channel tag", http.MethodDelete, "/api/v1/tags/" + shared.ID, "user:" + bobKratos, "", http.StatusForbidden},
		{"a channel tag needs membership to create", http.MethodPost, "/api/v1/tags", "user:kratos-003", `{"name":"x","scope":"channel","channel_id":"` + testChannelID + `"}`, http.StatusForbidden},
		{"a channel tag needs a channel", http.MethodPost, "/api/v1/tags", "user:" + aliceKratos, `{"name":"x","scope":"channel"}`, http.StatusBadRequest},
		{"a bad scope", http.MethodPost, "/api/v1/tags", "user:" + aliceKratos, `{"name":"x","scope":"team"}`, http.StatusBadRequest},
		{"a bad name", http.MethodPost, "/api/v1/tags", "user:" + aliceKratos, `{"name":"a/b"}`, http.StatusBadRequest},
		{"a name too long", http.MethodPost, "/api/v1/tags", "user:" + aliceKratos, `{"name":"` + strings.Repeat("n", model.MaxTagLength+1) + `"}`, http.StatusBadRequest},
		{"a non-member cannot see the channel's tags", http.MethodGet, "/api/v1/channels/" + testChannelID + "/tags", "user:kratos-003", "", http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, body := e.call(t, c.method, c.path, c.who, c.body, nil); code != c.want {
				t.Fatalf("got %d, want %d: %s", code, c.want, body)
			}
		})
	}
}

// A channel Tag applies in its own channel only.
func TestTags_ChannelTagStaysInItsChannel(t *testing.T) {
	e := newTagEnv(t)
	other := &model.Channel{ID: model.NewID(), TeamID: testTeamID, Name: "other", DisplayName: "Other", Type: "O", CreateAt: 1, UpdateAt: 1}
	e.ms.Channels.Seed(other)
	e.ms.Channels.SeedMember(&model.ChannelMember{ChannelID: other.ID, UserID: testUserID, CreateAt: 1})
	shared := e.mustCreateTag(t, "user:"+aliceKratos, "shared", model.TagScopeChannel, testChannelID, nil)
	if code, _ := e.call(t, http.MethodPost, "/api/v1/channels/"+other.ID+"/tags", "user:"+aliceKratos, `{"tag_id":"`+shared.ID+`"}`, nil); code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", code)
	}
}

func TestTags_PerItemLimit(t *testing.T) {
	e := newTagEnv(t)
	// Seeded, not created over HTTP: 64 requests would trip the per-user
	// rate limit.
	for i := range model.MaxTagsPerItem {
		tag := &model.Tag{ID: model.NewID(), Name: fmt.Sprintf("t%d", i), Scope: model.TagScopeChannel, ChannelID: testChannelID, OwnerID: testUserID}
		e.ms.Tags.Seed(tag)
		e.ms.Tags.SeedChannelTag(testChannelID, tag.ID)
	}
	over := e.mustCreateTag(t, "user:"+aliceKratos, "one too many", model.TagScopeChannel, testChannelID, nil)
	if code, _ := e.call(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", "user:"+aliceKratos, `{"tag_id":"`+over.ID+`"}`, nil); code != http.StatusBadRequest {
		t.Errorf("tag %d on one channel: got %d, want 400", model.MaxTagsPerItem+1, code)
	}
	// Bob's private tags are counted apart, so alice's full channel neither
	// stops him nor tells him anything.
	mine := e.mustCreateTag(t, "user:"+bobKratos, "mine", model.TagScopePrivate, "", nil)
	e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", "user:"+bobKratos, `{"tag_id":"`+mine.ID+`"}`)
}

func TestTags_PerOwnerLimit(t *testing.T) {
	e := newTagEnv(t)
	for i := range model.MaxTagsPerOwner {
		e.ms.Tags.Seed(&model.Tag{ID: model.NewID(), Name: fmt.Sprintf("seeded %d", i), Scope: model.TagScopePrivate, OwnerID: testUserID})
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags", "user:"+aliceKratos, `{"name":"one more"}`, nil); code != http.StatusBadRequest {
		t.Errorf("tag %d for one owner: got %d, want 400", model.MaxTagsPerOwner+1, code)
	}
}

func TestTags_FilterNeedsEveryTag(t *testing.T) {
	e := newTagEnv(t)
	a := e.mustCreateTag(t, "user:"+aliceKratos, "a", model.TagScopeChannel, testChannelID, nil)
	b := e.mustCreateTag(t, "user:"+aliceKratos, "b", model.TagScopeChannel, testChannelID, nil)
	// a on the channel, b on one of its posts: the channel has both.
	e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", "user:"+aliceKratos, `{"tag_id":"`+a.ID+`"}`)
	_, body := e.call(t, http.MethodPost, "/api/v1/tags/channels", "user:"+aliceKratos, `{"tag_ids":["`+a.ID+`","`+b.ID+`"]}`, nil)
	if strings.Contains(body, testChannelID) {
		t.Errorf("matched with only one of two tags: %s", body)
	}
	e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testReplyPost+"/tags", "user:"+aliceKratos, `{"tag_id":"`+b.ID+`"}`)
	_, body = e.call(t, http.MethodPost, "/api/v1/tags/channels", "user:"+aliceKratos, `{"tag_ids":["`+a.ID+`","`+b.ID+`"]}`, nil)
	if !strings.Contains(body, testChannelID) {
		t.Errorf("no match with both tags: %s", body)
	}
	nine := `"` + strings.TrimSuffix(strings.Repeat(model.NewID()+`","`, model.MaxTagFilter+1), `","`) + `"`
	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags/channels", "user:"+aliceKratos, `{"tag_ids":[`+nine+`]}`, nil); code != http.StatusBadRequest {
		t.Errorf("a filter of %d tags: got %d, want 400", model.MaxTagFilter+1, code)
	}
}

// ─── The reserved profile/ prefix ────────────────────────────────

func TestTags_ProfilePrefixIsReserved(t *testing.T) {
	e := newTagEnv(t)
	alice := "user:" + aliceKratos
	halo := "client:" + haloClientID
	forAlice := obo(testUserID)

	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags", alice, `{"name":"profile/coder"}`, nil); code != http.StatusForbidden {
		t.Errorf("a person creating a profile/ tag: got %d, want 403", code)
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags", alice, `{"name":"PROFILE/coder"}`, nil); code != http.StatusForbidden {
		t.Errorf("a person creating PROFILE/: got %d, want 403", code)
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags", halo, `{"name":"profile/coder","scope":"channel","channel_id":"`+testChannelID+`"}`, forAlice); code != http.StatusBadRequest {
		t.Errorf("a channel profile/ tag on behalf: got %d, want 400", code)
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/tags", "client:"+otherClientID, `{"name":"profile/coder"}`, nil); code != http.StatusForbidden {
		t.Errorf("a client acting for itself: got %d, want 403", code)
	}

	profile := e.mustCreateTag(t, halo, "profile/coder", model.TagScopePrivate, "", forAlice)
	if profile.OwnerID != testUserID || profile.Scope != model.TagScopePrivate {
		t.Fatalf("on-behalf-of created %+v, want alice's private tag", profile)
	}
	apply := `{"tag_id":"` + profile.ID + `"}`
	if code, _ := e.call(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", alice, apply, nil); code != http.StatusForbidden {
		t.Errorf("the owner applying her own profile/ tag directly: got %d, want 403", code)
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", alice, apply, nil); code != http.StatusForbidden {
		t.Errorf("the owner applying it to a channel directly: got %d, want 403", code)
	}
	if code, body := e.call(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", halo, apply, forAlice); code != http.StatusOK {
		t.Fatalf("on-behalf-of apply: %d %s", code, body)
	}
	if code, _ := e.call(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", halo, apply, obo(extraUserID)); code != http.StatusNotFound {
		t.Errorf("applying alice's profile/ tag on bob's behalf: got %d, want 404", code)
	}
	if code, _ := e.call(t, http.MethodPut, "/api/v1/tags/"+profile.ID, alice, `{"name":"coder"}`, nil); code != http.StatusForbidden {
		t.Errorf("renaming a profile/ tag: got %d, want 403", code)
	}
	plain := e.mustCreateTag(t, alice, "plain", model.TagScopePrivate, "", nil)
	if code, _ := e.call(t, http.MethodPut, "/api/v1/tags/"+plain.ID, alice, `{"name":"profile/fake"}`, nil); code != http.StatusForbidden {
		t.Errorf("renaming a tag to profile/: got %d, want 403", code)
	}
	// The owner may still remove it.
	e.mustOK(t, http.MethodDelete, "/api/v1/posts/"+testRootPost+"/tags/"+profile.ID, alice, "")
}
