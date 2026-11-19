package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store/storetest"
)

// Tests for the HALO bridge prerequisites: posting on behalf of a person
// (P-CHIT-1), props.provenance (P-CHIT-3) and stream_status (P-CHIT-4). They
// run the whole router, so AuthExtract, OpenAPI validation and the handlers
// all take part.

const (
	haloClientID    = "halod-client"
	otherClientID   = "other-client"
	haloAgentID     = "019421a0-0000-7000-8000-0000000000a1"
	otherAgentID    = "019421a0-0000-7000-8000-0000000000a2"
	unknownPersonID = "019421a0-0000-7000-8000-0000000000ff"
)

type bridgeEnv struct {
	a      *app.App
	ms     *storetest.Store
	router http.Handler
	audit  string
}

func newBridgeEnv(t *testing.T) *bridgeEnv {
	t.Helper()
	a, ms, cleanup := setupTestApp(t)
	t.Cleanup(cleanup)
	for _, u := range []*model.User{
		{ID: haloAgentID, Username: "halod", DisplayName: "halod", ActorType: model.ActorTypeAgent, OAuthClientID: haloClientID},
		{ID: otherAgentID, Username: "other-agent", DisplayName: "Other", ActorType: model.ActorTypeAgent, OAuthClientID: otherClientID},
	} {
		u.Roles, u.CreateAt, u.UpdateAt = "system_user", 1000, 1000
		ms.Users.Seed(u)
	}
	// Both agents are members of the channel, so a refusal below is about
	// on-behalf-of, not about membership.
	for _, id := range []string{haloAgentID, otherAgentID} {
		ms.Channels.SeedMember(&model.ChannelMember{ChannelID: testChannelID, UserID: id, CreateAt: 1000})
	}
	a.Config.OnBehalfOfClients = []string{haloClientID}

	audit := filepath.Join(t.TempDir(), "audit.log")
	al, err := command.NewAuditLogger(audit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeOK(t, al, "audit logger") })
	a.AuditLogger = al
	return &bridgeEnv{a: a, ms: ms, router: New(a), audit: audit}
}

// do sends a request through the router. who is either "client:<id>" or
// "user:<kratos id>"; headers are extra request headers.
func (e *bridgeEnv) do(t *testing.T, method, path, who, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	switch kind, id, _ := strings.Cut(who, ":"); kind {
	case "client":
		r.Header.Set("X-Client-Id", id)
	case "user":
		r.Header.Set("X-User-Id", id)
	}
	for k, v := range headers {
		r.Header[k] = []string{v}
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

func postBody(content, props string) string {
	b := `{"channel_id":"` + testChannelID + `","content":"` + content + `"`
	if props != "" {
		b += `,"props":` + props
	}
	return b + "}"
}

func obo(id string) map[string]string { return map[string]string{OnBehalfOfHeader: id} }

// ─── P-CHIT-1: on behalf of ──────────────────────────────────────

func TestOnBehalfOf_AllowlistedClientPostsAsThePerson(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+haloClientID, postBody("hi from halod", ""), obo(testUserID))
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var post model.Post
	decodeJSON(t, w.Body, &post)
	if post.UserID != testUserID {
		t.Errorf("author %q, want the principal %q", post.UserID, testUserID)
	}
	rec, ok := post.Props[model.PropOnBehalfOf].(map[string]any)
	if !ok || rec["actor_id"] != haloAgentID {
		t.Errorf("props.on_behalf_of = %v, want actor_id %s", post.Props[model.PropOnBehalfOf], haloAgentID)
	}

	// The audit record names both.
	data, err := os.ReadFile(e.audit)
	if err != nil {
		t.Fatal(err)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &line); err != nil {
		t.Fatalf("audit log %q: %v", data, err)
	}
	if line["msg"] != "post_on_behalf_of" || line["actor_id"] != haloAgentID || line["client_id"] != haloClientID ||
		line["principal_id"] != testUserID || line["post_id"] != post.ID {
		t.Errorf("audit record = %v", line)
	}
}

// The post is authorized as the person: a principal who is not a member of
// the channel is refused, although the client itself is one.
func TestOnBehalfOf_ChecksThePrincipalsMembership(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+haloClientID, postBody("hi", ""), obo(extraUserID))
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
	}
}

func TestOnBehalfOf_Refusals(t *testing.T) {
	cases := []struct {
		name, who, principal string
		want                 int
	}{
		{"client not on the allowlist", "client:" + otherClientID, testUserID, http.StatusForbidden},
		{"a person's session", "user:" + testKratosID, extraUserID, http.StatusForbidden},
		{"empty principal", "client:" + haloClientID, "", http.StatusBadRequest},
		{"malformed principal", "client:" + haloClientID, "not-a-uuid", http.StatusBadRequest},
		{"unknown principal", "client:" + haloClientID, unknownPersonID, http.StatusBadRequest},
		{"principal is an agent", "client:" + haloClientID, otherAgentID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBridgeEnv(t)
			w := e.do(t, http.MethodPost, "/api/v1/posts", tc.who, postBody("hi", ""), obo(tc.principal))
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			list, err := e.ms.Posts.GetPostsForChannel(t.Context(), testChannelID, model.GetPostsOptions{PerPage: 100})
			if err != nil {
				t.Fatalf("list posts: %v", err)
			}
			if len(list.Order) != 2 {
				t.Fatalf("%d posts in the channel after a refusal, want the 2 seeded", len(list.Order))
			}
		})
	}
}

// Only posting and stream_status act on behalf of a person; anywhere else
// the header is refused rather than silently ignored.
func TestOnBehalfOf_RefusedOnOtherRoutes(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/channels/"+testChannelID+"/posts", "client:"+haloClientID, "", obo(testUserID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

// A client cannot forge the record: props.on_behalf_of is the server's.
func TestOnBehalfOf_ClientCannotForgeTheRecord(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodPost, "/api/v1/posts", "user:"+testKratosID,
		postBody("hi", `{"on_behalf_of":{"actor_id":"`+haloAgentID+`"}}`), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var post model.Post
	decodeJSON(t, w.Body, &post)
	if _, ok := post.Props[model.PropOnBehalfOf]; ok {
		t.Fatalf("a forged on_behalf_of was stored: %v", post.Props)
	}
}

func TestCheckOnBehalfOfClients(t *testing.T) {
	actors := []app.MachineActor{{ClientID: haloClientID}}
	if err := app.CheckOnBehalfOfClients([]string{haloClientID}, actors); err != nil {
		t.Fatal(err)
	}
	if err := app.CheckOnBehalfOfClients([]string{"typo-client"}, actors); err == nil {
		t.Fatal("an allowlisted client that is not a machine actor was accepted")
	}
}

// ─── P-CHIT-3: provenance ────────────────────────────────────────

const goodProvenance = `{"provenance":{` +
	`"tool_ids":["019421a0-0000-7000-8000-0000000000b1"],` +
	`"okf_namespace_ids":["019421a0-0000-7000-8000-0000000000b2"],` +
	`"okf_blob_sha256":"` + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" + `",` +
	`"model":"llama3.1:8b",` +
	`"attachment_ids":[]}}`

func TestProvenance_AgentAndOnBehalfOfMaySetIt(t *testing.T) {
	e := newBridgeEnv(t)
	if w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+otherClientID, postBody("agent reply", goodProvenance), nil); w.Code != http.StatusCreated {
		t.Fatalf("agent: got %d: %s", w.Code, w.Body.String())
	}
	if w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+haloClientID, postBody("for a person", goodProvenance), obo(testUserID)); w.Code != http.StatusCreated {
		t.Fatalf("on behalf of: got %d: %s", w.Code, w.Body.String())
	}
}

func TestProvenance_PersonMayNotSetIt(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodPost, "/api/v1/posts", "user:"+testKratosID, postBody("I used no tools", goodProvenance), nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403: %s", w.Code, w.Body.String())
	}
}

// Each malformed value is a 400. They go to app.CreatePostAs directly as
// well as through the router, because the OpenAPI validator would refuse
// some of them first, and the app must hold the line without it.
func TestProvenance_MalformedIsRefused(t *testing.T) {
	uuid := `"019421a0-0000-7000-8000-0000000000b1"`
	cases := map[string]string{
		"not an object":       `"tools"`,
		"unknown field":       `{"tools":[]}`,
		"ids not a list":      `{"tool_ids":` + uuid + `}`,
		"id not a UUID":       `{"tool_ids":["tool-1"]}`,
		"unhyphenated UUID":   `{"attachment_ids":["019421a00000700080000000000000b1"]}`,
		"id not a string":     `{"okf_namespace_ids":[42]}`,
		"sha256 too short":    `{"okf_blob_sha256":"abc"}`,
		"sha256 upper case":   `{"okf_blob_sha256":"` + strings.Repeat("A", 64) + `"}`,
		"model empty":         `{"model":""}`,
		"model too long":      `{"model":"` + strings.Repeat("m", 201) + `"}`,
		"too many attachment": `{"attachment_ids":[` + strings.TrimSuffix(strings.Repeat(uuid+",", 101), ",") + `]}`,
	}
	for name, prov := range cases {
		t.Run(name, func(t *testing.T) {
			e := newBridgeEnv(t)
			w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+otherClientID, postBody("x", `{"provenance":`+prov+`}`), nil)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("router: got %d, want 400: %s", w.Code, w.Body.String())
			}
			var props map[string]any
			if err := json.Unmarshal([]byte(`{"provenance":`+prov+`}`), &props); err != nil {
				t.Fatal(err)
			}
			_, err := e.a.CreatePost(t.Context(), &model.Post{ChannelID: testChannelID, UserID: otherAgentID, Content: "x", Props: props})
			var appErr *model.AppError
			if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusBadRequest {
				t.Fatalf("app: got %v, want a 400", err)
			}
		})
	}
}

func TestProvenance_OnEdit(t *testing.T) {
	e := newBridgeEnv(t)
	// A person may not add provenance to their own post, nor remove it.
	for _, patch := range []string{goodProvenance, `{"provenance":null}`} {
		w := e.do(t, http.MethodPut, "/api/v1/posts/"+testRootPost, "user:"+testKratosID, `{"props":`+patch+`}`, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("person edit %s: got %d, want 403: %s", patch, w.Code, w.Body.String())
		}
	}

	// An agent may, on its own post, but only a well-formed value.
	w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+otherClientID, postBody("draft", ""), nil)
	var post model.Post
	decodeJSON(t, w.Body, &post)
	path := "/api/v1/posts/" + post.ID
	if w := e.do(t, http.MethodPut, path, "client:"+otherClientID, `{"props":{"provenance":{"model":""}}}`, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad agent edit: got %d, want 400: %s", w.Code, w.Body.String())
	}
	if w := e.do(t, http.MethodPut, path, "client:"+otherClientID, `{"props":`+goodProvenance+`}`, nil); w.Code != http.StatusOK {
		t.Fatalf("agent edit: got %d: %s", w.Code, w.Body.String())
	}
}

// An edit cannot touch the on-behalf-of record.
func TestOnBehalfOf_EditKeepsTheRecord(t *testing.T) {
	e := newBridgeEnv(t)
	w := e.do(t, http.MethodPost, "/api/v1/posts", "client:"+haloClientID, postBody("hi", ""), obo(testUserID))
	var post model.Post
	decodeJSON(t, w.Body, &post)
	w = e.do(t, http.MethodPut, "/api/v1/posts/"+post.ID, "user:"+testKratosID, `{"props":{"on_behalf_of":null}}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	decodeJSON(t, w.Body, &post)
	if _, ok := post.Props[model.PropOnBehalfOf]; !ok {
		t.Fatal("an edit removed props.on_behalf_of")
	}
}

// ─── P-CHIT-4: stream_status ─────────────────────────────────────

func streamPath() string { return "/api/v1/channels/" + testChannelID + "/stream_status" }

// A member's stream_status reaches the channel's WebSocket clients, and
// nothing is stored: no post, no message count.
func TestStreamStatus_RelayedAndNotStored(t *testing.T) {
	e := newBridgeEnv(t)
	h := handleWebSocket(e.a)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, authedRequest(r, testUser()))
	}))
	defer srv.Close()
	conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer closeOK(t, conn, "websocket")
	if err = resp.Body.Close(); err != nil {
		t.Errorf("close handshake response body: %v", err)
	}

	before, err := e.ms.Channels.Get(t.Context(), testChannelID)
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	w := e.do(t, http.MethodPost, streamPath(), "client:"+haloClientID, `{"status":"started","stream_id":"reply-1"}`, obo(testUserID))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	readDeadlineOK(t, conn, 2*time.Second)
	var ev model.WebSocketEvent
	if err = conn.ReadJSON(&ev); err != nil {
		t.Fatalf("read: %v", err)
	}
	if ev.Event != model.WebSocketEventStreamStatus || ev.Data["status"] != "started" || ev.Data["user_id"] != testUserID ||
		ev.Data["actor_id"] != haloAgentID || ev.Data["stream_id"] != "reply-1" || ev.Broadcast.ChannelID != testChannelID {
		t.Fatalf("got %+v", ev)
	}

	list, err := e.ms.Posts.GetPostsForChannel(t.Context(), testChannelID, model.GetPostsOptions{PerPage: 100})
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if len(list.Order) != 2 {
		t.Errorf("%d posts in the channel, want the 2 seeded: stream_status was stored", len(list.Order))
	}
	after, err := e.ms.Channels.Get(t.Context(), testChannelID)
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	if after.TotalMsgCount != before.TotalMsgCount || after.LastPostAt != before.LastPostAt {
		t.Errorf("channel counters moved: %+v -> %+v", before, after)
	}
}

func TestStreamStatus_Refusals(t *testing.T) {
	e := newBridgeEnv(t)
	cases := []struct {
		name, who, body string
		headers         map[string]string
		want            int
	}{
		{"not a member", "user:kratos-002", `{"status":"started"}`, nil, http.StatusForbidden},
		{"unknown status", "user:" + testKratosID, `{"status":"typing"}`, nil, http.StatusBadRequest},
		{"bad stream_id", "user:" + testKratosID, `{"status":"progress","stream_id":"a b"}`, nil, http.StatusBadRequest},
		{"client not allowlisted", "client:" + otherClientID, `{"status":"started"}`, obo(testUserID), http.StatusForbidden},
		{"principal not a member", "client:" + haloClientID, `{"status":"started"}`, obo(extraUserID), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := e.do(t, http.MethodPost, streamPath(), tc.who, tc.body, tc.headers); w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestStreamStatus_RateLimited(t *testing.T) {
	e := newBridgeEnv(t)
	codes := []int{}
	for range streamStatusBurst + 1 {
		codes = append(codes, e.do(t, http.MethodPost, streamPath(), "user:"+testKratosID, `{"status":"progress"}`, nil).Code)
	}
	for i, c := range codes[:streamStatusBurst] {
		if c != http.StatusOK {
			t.Fatalf("event %d: got %d within the burst", i, c)
		}
	}
	if last := codes[streamStatusBurst]; last != http.StatusTooManyRequests {
		t.Fatalf("event past the burst: got %d, want 429", last)
	}
}
