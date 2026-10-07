package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

// privateRootPost is a root post in privateChannelID, which only testUser
// can read.
const privateRootPost = "019421a0-0000-7000-8000-000000000032"

func seedPrivateRoot(ms *mockStore) {
	seedPrivateChannel(ms)
	ms.post.seed(&model.Post{
		ID: privateRootPost, ChannelID: privateChannelID, UserID: testUserID,
		Content: "private plans", CreateAt: 2000, UpdateAt: 2000,
	})
}

func postJSON(t *testing.T, handler http.HandlerFunc, body string, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestCreatePost_RootMustBeInTheSameChannel(t *testing.T) {
	cases := []struct {
		name   string
		rootID string
	}{
		// alice is in neither channel; testUser is in both but replies
		// across channels all the same.
		{"root in another channel", privateRootPost},
		{"root is itself a reply", testReplyPost},
		{"root does not exist", "019421a0-0000-7000-8000-0000000000ff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()
			seedPrivateRoot(ms)

			body := `{"channel_id":"` + testChannelID + `","root_id":"` + tc.rootID + `","content":"reply"}`
			w := postJSON(t, createPost(a), body, testUser())
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}
			if _, err := ms.thread.Get(t.Context(), privateRootPost); err == nil {
				t.Fatal("a rejected reply still created a thread on the private root")
			}
		})
	}
}

func TestCreatePost_IgnoresServerOwnedFields(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	const forgedID = "019421a0-0000-7000-8000-0000000000ee"
	body := `{"id":"` + forgedID + `","channel_id":"` + testChannelID + `","content":"hi",` +
		`"create_at":99999999999999,"is_pinned":true,"type":"system_join","delete_at":7,` +
		`"props":{"mentions":["` + extraUserID + `"],"agent_session":"s1"}}`
	w := postJSON(t, createPost(a), body, testUser())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var p model.Post
	decodeJSON(t, w.Body, &p)
	if p.ID == forgedID || p.CreateAt == 99999999999999 || p.IsPinned || p.Type != "" || p.DeleteAt != 0 {
		t.Fatalf("server-owned fields were taken from the client: %+v", p)
	}
	if _, ok := p.Props["mentions"]; ok {
		t.Fatal("a client-supplied mentions prop was kept")
	}
	if p.Props["agent_session"] != "s1" {
		t.Fatal("ordinary client props must be kept")
	}
}

func TestUpdatePost_ReturnsTheWholePost(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()
	ms.post.byID[testReplyPost].Props = map[string]any{"keep": "me", "drop": "me"}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/",
		strings.NewReader(`{"content":"edited","props":{"drop":null,"add":1}}`))
	r = withChiParam(r, "id", testReplyPost)
	r = authedRequest(r, aliceUser())
	w := httptest.NewRecorder()
	updatePost(a).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var p model.Post
	decodeJSON(t, w.Body, &p)
	if p.Content != "edited" || p.RootID != testRootPost || p.CreateAt != 3000 || p.ChannelID != testChannelID {
		t.Fatalf("edit lost fields it did not touch: %+v", p)
	}
	if p.Props["keep"] != "me" || p.Props["add"] == nil {
		t.Fatalf("props were not merged: %v", p.Props)
	}
	if _, ok := p.Props["drop"]; ok {
		t.Fatalf("a null prop should remove the key: %v", p.Props)
	}
}

func TestUpdatePost_RejectsEmptyContent(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(`{"content":""}`))
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()
	updatePost(a).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

// Following a thread puts its root in the follower's inbox, and neither the
// follow nor the mark-read route checked that the caller could read the post.
func TestThreadRoutes_RequireChannelAccess(t *testing.T) {
	routes := []struct {
		name    string
		handler func(*app.App) http.HandlerFunc
		method  string
		body    string
	}{
		{"follow", updateThreadFollowing, http.MethodPut, `{"following":true}`},
		{"mark read", markThreadAsRead, http.MethodPut, ``},
	}
	cases := []struct {
		name   string
		postID string
		user   *model.User
		want   int
	}{
		{"non-member of a private channel", privateRootPost, aliceUser(), http.StatusForbidden},
		{"a reply is not a thread", testReplyPost, testUser(), http.StatusBadRequest},
	}
	for _, rt := range routes {
		for _, tc := range cases {
			t.Run(rt.name+"/"+tc.name, func(t *testing.T) {
				a, ms, cleanup := setupTestApp(t)
				defer cleanup()
				seedPrivateRoot(ms)

				r := httptest.NewRequestWithContext(t.Context(), rt.method, "/", strings.NewReader(rt.body))
				r = withChiParams(r, map[string]string{"id": tc.postID, "team_id": testTeamID})
				r = authedRequest(r, tc.user)
				w := httptest.NewRecorder()
				rt.handler(a).ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
				}
				if _, err := ms.thread.GetMembership(t.Context(), tc.postID, tc.user.ID); err == nil && tc.want == http.StatusForbidden {
					t.Fatal("a rejected follow still created a thread membership")
				}
			})
		}
	}
}
