package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

func setupTestClient(t *testing.T, mux *http.ServeMux) (api.ChitClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-token")
	return client, srv
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.AppError{ID: "test", Message: msg, StatusCode: status})
}

func TestGetMe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Session-Token") != "test-token" {
			writeError(w, 401, "unauthorized")
			return
		}
		writeJSON(w, model.User{ID: "u1", Username: "alice"})
	})
	client, _ := setupTestClient(t, mux)

	u, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "u1" || u.Username != "alice" {
		t.Errorf("unexpected user: %+v", u)
	}
}

func TestGetMyTeams(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me/teams", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.Team{{ID: "t1", Name: "eng"}})
	})
	client, _ := setupTestClient(t, mux)

	teams, err := client.GetMyTeams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) != 1 || teams[0].ID != "t1" {
		t.Errorf("unexpected teams: %+v", teams)
	}
}

func TestGetMyChannels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me/teams/{id}/channels", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id != "t1" {
			writeError(w, 404, "not found")
			return
		}
		writeJSON(w, []*model.Channel{{ID: "c1", Name: "general"}})
	})
	client, _ := setupTestClient(t, mux)

	channels, err := client.GetMyChannels(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != "c1" {
		t.Errorf("unexpected channels: %+v", channels)
	}
}

func TestGetChannelPosts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/channels/{id}/posts", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, model.PostList{Order: []*model.Post{{ID: "p1", Content: "hello"}}})
	})
	client, _ := setupTestClient(t, mux)

	pl, err := client.GetChannelPosts(context.Background(), "c1", 0, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Order) != 1 || pl.Order[0].ID != "p1" {
		t.Errorf("unexpected posts: %+v", pl)
	}
}

func TestCreatePost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/posts", func(w http.ResponseWriter, r *http.Request) {
		var p model.Post
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		p.ID = "new-post-id"
		writeJSON(w, p)
	})
	client, _ := setupTestClient(t, mux)

	post, err := client.CreatePost(context.Background(), &model.Post{ChannelID: "c1", Content: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "new-post-id" {
		t.Errorf("unexpected post ID: %s", post.ID)
	}
}

func TestGetPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/posts/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, model.Post{ID: "p1", Content: "hello"})
	})
	client, _ := setupTestClient(t, mux)

	p, err := client.GetPost(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p1" {
		t.Errorf("unexpected post: %+v", p)
	}
}

func TestPinPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/posts/{id}/pin", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.PinPost(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
}

func TestUnpinPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/posts/{id}/unpin", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.UnpinPost(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
}

func TestGetThread(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/posts/{id}/thread", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, model.PostList{
			Order: []*model.Post{{ID: "p1"}, {ID: "r1", RootID: "p1"}},
		})
	})
	client, _ := setupTestClient(t, mux)

	pl, err := client.GetThread(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Order) != 2 {
		t.Errorf("unexpected thread: %+v", pl)
	}
}

func TestGetCommands(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/commands", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.Command{{ID: "c1", Slug: "remind"}})
	})
	client, _ := setupTestClient(t, mux)

	cmds, err := client.GetCommands(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 || cmds[0].Slug != "remind" {
		t.Errorf("unexpected commands: %+v", cmds)
	}
}

func TestViewChannel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/{id}/members/me/view", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.ViewChannel(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
}

func TestGetUsersByIDs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/users/ids", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		writeJSON(w, []*model.User{{ID: ids[0], Username: "alice"}})
	})
	client, _ := setupTestClient(t, mux)

	users, err := client.GetUsersByIDs(context.Background(), []string{"u1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != "u1" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestSearchPosts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/{id}/posts/search", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, model.PostList{Order: []*model.Post{{ID: "p1", Content: "match"}}})
	})
	client, _ := setupTestClient(t, mux)

	pl, err := client.SearchPosts(context.Background(), "c1", "match", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Order) != 1 {
		t.Errorf("unexpected search results: %+v", pl)
	}
}

func TestGetChannelMembers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/channels/{id}/members", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.ChannelMember{
			{ChannelID: "c1", UserID: "u1", MsgCount: 5},
			{ChannelID: "c1", UserID: "u2", MsgCount: 3},
		})
	})
	client, _ := setupTestClient(t, mux)

	members, err := client.GetChannelMembers(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	if members[0].UserID != "u1" || members[0].MsgCount != 5 {
		t.Errorf("unexpected member: %+v", members[0])
	}
}

func TestGetMyDirectChannels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me/channels/direct", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.Channel{{ID: "dm1", Name: "u1__u2", Type: "D"}})
	})
	client, _ := setupTestClient(t, mux)

	channels, err := client.GetMyDirectChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != "dm1" {
		t.Errorf("unexpected channels: %+v", channels)
	}
}

func TestSearchUsers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("term")
		if term != "ali" {
			writeError(w, 400, "bad term")
			return
		}
		writeJSON(w, []*model.User{{ID: "u1", Username: "alice"}})
	})
	client, _ := setupTestClient(t, mux)

	users, err := client.SearchUsers(context.Background(), "ali", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestCreateGroupChannel_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/group", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		if len(ids) < 3 {
			writeError(w, 400, "need at least 3 users")
			return
		}
		writeJSON(w, model.Channel{ID: "g1", Type: "G", Name: "u1__u2__u3"})
	})
	client, _ := setupTestClient(t, mux)

	ch, err := client.CreateGroupChannel(context.Background(), []string{"u1", "u2", "u3"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "g1" || ch.Type != "G" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

func TestCreateChannel_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels", func(w http.ResponseWriter, r *http.Request) {
		var ch model.Channel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		ch.ID = "new-ch-id"
		writeJSON(w, ch)
	})
	client, _ := setupTestClient(t, mux)

	ch, err := client.CreateChannel(context.Background(), &model.Channel{
		TeamID:      "t1",
		Name:        "deployments",
		DisplayName: "Deployments",
		Type:        "O",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "new-ch-id" || ch.Name != "deployments" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

func TestCreateDirectChannel_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/direct", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, model.Channel{ID: "dm1", Type: "D", Name: "u1__u2"})
	})
	client, _ := setupTestClient(t, mux)

	ch, err := client.CreateDirectChannel(context.Background(), "u1", "u2")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "dm1" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

func TestGetAllTags(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tags", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.Tag{{ID: "t1", Name: "urgent"}})
	})
	client, _ := setupTestClient(t, mux)

	tags, err := client.GetAllTags(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "urgent" {
		t.Errorf("unexpected tags: %+v", tags)
	}
}

func TestCreateTag(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		writeJSON(w, model.Tag{ID: "t1", Name: body["name"]})
	})
	client, _ := setupTestClient(t, mux)

	tag, err := client.CreateTag(context.Background(), "urgent")
	if err != nil {
		t.Fatal(err)
	}
	if tag.ID != "t1" || tag.Name != "urgent" {
		t.Errorf("unexpected tag: %+v", tag)
	}
}

func TestGetTagsForPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/posts/{id}/tags", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []*model.Tag{{ID: "t1", Name: "urgent"}})
	})
	client, _ := setupTestClient(t, mux)

	tags, err := client.GetTagsForPost(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "urgent" {
		t.Errorf("unexpected tags: %+v", tags)
	}
}

func TestAddTagToPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/posts/{id}/tags", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.AddTagToPost(context.Background(), "p1", "t1"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveTagFromPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/posts/{id}/tags/{tag_id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.RemoveTagFromPost(context.Background(), "p1", "t1"); err != nil {
		t.Fatal(err)
	}
}

func TestAddChannelMember(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/channels/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		if body["user_id"] != "u2" {
			writeError(w, 400, "wrong user_id")
			return
		}
		w.WriteHeader(200)
	})
	client, _ := setupTestClient(t, mux)

	if err := client.AddChannelMember(context.Background(), "c1", "u2"); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientWithTokenFn_DynamicToken(t *testing.T) {
	token := "initial-token"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Session-Token")
		writeJSON(w, model.User{ID: "u1", Username: got})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := api.NewClientWithTokenFn(srv.URL, func() string { return token }, "X-Session-Token")

	u, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "initial-token" {
		t.Errorf("expected initial-token, got %q", u.Username)
	}

	// Update token
	token = "updated-token"
	u, err = client.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "updated-token" {
		t.Errorf("expected updated-token, got %q", u.Username)
	}
}

func TestGetMe_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, 401, "unauthorized")
	})
	client, _ := setupTestClient(t, mux)

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
}

// The term goes into a query string, so spaces and ampersands have to be
// escaped or the parameter splits and the server searches for something else.
func TestSearchUsers_EscapesTerm(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := api.NewClient(srv.URL, "token")

	if _, err := c.SearchUsers(context.Background(), "ann & bob", 0, 10); err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	parsed, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("server received an unparseable query %q: %v", gotQuery, err)
	}
	if got := parsed.Get("term"); got != "ann & bob" {
		t.Errorf("term = %q, want %q (raw query: %s)", got, "ann & bob", gotQuery)
	}
}

// The server treats absent fields as "leave alone", so the client must send
// only what changed. Marshaling the whole user would rename the handle every
// time someone edited their display name.
func TestUpdateMe_SendsOnlyTheChangedField(t *testing.T) {
	var body map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, 400, "bad body")
			return
		}
		writeJSON(w, model.User{ID: "u1", Username: "alice", DisplayName: "Alice Anderson"})
	})
	client, _ := setupTestClient(t, mux)

	u, err := client.UpdateMe(context.Background(), &model.User{DisplayName: "Alice Anderson"})
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "Alice Anderson" {
		t.Errorf("display name = %q", u.DisplayName)
	}
	// Absent, not empty: the schema's username pattern does not match "",
	// so sending the key at all is a 400 rather than a no-op.
	if got, ok := body["username"]; ok {
		t.Errorf("sent username %q on a display-name change; the schema rejects it", got)
	}
}

func TestUpdateMe_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/users/me", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, 409, "username already taken")
	})
	client, _ := setupTestClient(t, mux)

	if _, err := client.UpdateMe(context.Background(), &model.User{Username: "bob"}); err == nil {
		t.Fatal("expected an error for a taken username")
	}
}

// With no timeout, a server that accepted a request and never answered left
// the command waiting forever: "Loading messages…" never cleared, and paging
// stayed locked for the channel.
func TestClient_GivesUpOnAServerThatNeverAnswers(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	c := api.NewClient(srv.URL, "token")
	if api.DefaultTimeout <= 0 {
		t.Fatal("no default timeout")
	}
	api.SetTimeout(c, 50*time.Millisecond)

	done := make(chan error, 1)
	go func() { _, err := c.GetMe(context.Background()); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a request that never got an answer succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the request never gave up")
	}
}

// Each call reaches the endpoint the server serves it on, with the body the
// server expects. These had no test at all.
func TestClient_Endpoints(t *testing.T) {
	tests := []struct {
		name     string
		route    string // "METHOD path", as http.ServeMux patterns write it
		reply    any
		call     func(api.ChitClient) error
		wantBody string // JSON the request must carry; "" for none
	}{
		{
			name:  "threads",
			route: "GET /api/v1/users/me/teams/t1/threads",
			reply: model.UserThreadList{Total: 1},
			call: func(c api.ChitClient) error {
				list, err := c.GetMyThreads(context.Background(), "t1", 2, 25)
				if err == nil && list.Total != 1 {
					return fmt.Errorf("total = %d, want 1", list.Total)
				}
				return err
			},
		},
		{
			name:  "search everywhere",
			route: "POST /api/v1/posts/search",
			reply: model.PostList{},
			call: func(c api.ChitClient) error {
				_, err := c.SearchPostsEverywhere(context.Background(), "deploy", nil)
				return err
			},
			wantBody: `{"terms":"deploy"}`,
		},
		{
			name:  "search everywhere by tag",
			route: "POST /api/v1/posts/search",
			reply: model.PostList{},
			call: func(c api.ChitClient) error {
				_, err := c.SearchPostsEverywhere(context.Background(), "", []string{"t9"})
				return err
			},
			wantBody: `{"tag_ids":["t9"],"terms":""}`,
		},
		{
			name:  "edit",
			route: "PUT /api/v1/posts/p1",
			reply: model.Post{ID: "p1", Content: "new"},
			call: func(c api.ChitClient) error {
				p, err := c.UpdatePost(context.Background(), "p1", "new")
				if err == nil && p.Content != "new" {
					return fmt.Errorf("content = %q", p.Content)
				}
				return err
			},
			wantBody: `{"content":"new"}`,
		},
		{
			name:  "delete",
			route: "DELETE /api/v1/posts/p1",
			call:  func(c api.ChitClient) error { return c.DeletePost(context.Background(), "p1") },
		},
		{
			name:  "tags for posts",
			route: "POST /api/v1/tags/posts",
			reply: map[string][]*model.Tag{"p1": {{ID: "t1", Name: "urgent"}}},
			call: func(c api.ChitClient) error {
				tags, err := c.GetTagsForPosts(context.Background(), []string{"p1", "p2"})
				if err == nil && (len(tags["p1"]) != 1 || tags["p1"][0].Name != "urgent") {
					return fmt.Errorf("tags = %v", tags)
				}
				return err
			},
			wantBody: `{"post_ids":["p1","p2"]}`,
		},
		{
			name:  "my memberships in a team",
			route: "GET /api/v1/users/me/teams/t1/channels/members",
			reply: []*model.ChannelMember{{ChannelID: "c1", UserID: "u1", MentionCount: 2}},
			call: func(c api.ChitClient) error {
				ms, err := c.GetMyChannelMembers(context.Background(), "t1")
				if err == nil && (len(ms) != 1 || ms[0].MentionCount != 2) {
					return fmt.Errorf("members = %v", ms)
				}
				return err
			},
		},
		{
			name:  "leave",
			route: "DELETE /api/v1/channels/c1/members/u1",
			call:  func(c api.ChitClient) error { return c.RemoveChannelMember(context.Background(), "c1", "u1") },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body string
			hit := false
			mux := http.NewServeMux()
			mux.HandleFunc(tc.route, func(w http.ResponseWriter, r *http.Request) {
				hit = true
				raw, _ := io.ReadAll(r.Body)
				body = strings.TrimSpace(string(raw))
				if tc.reply != nil {
					writeJSON(w, tc.reply)
				}
			})
			client, _ := setupTestClient(t, mux)

			if err := tc.call(client); err != nil {
				t.Fatal(err)
			}
			if !hit {
				t.Fatalf("%s was not requested", tc.route)
			}
			if body != tc.wantBody {
				t.Errorf("body = %s, want %s", body, tc.wantBody)
			}
		})
	}
}

// With no posts there is nothing to ask for, and no request is made.
func TestClient_GetTagsForNoPostsMakesNoRequest(t *testing.T) {
	client, _ := setupTestClient(t, http.NewServeMux()) // any request would 404
	tags, err := client.GetTagsForPosts(context.Background(), nil)
	if err != nil || len(tags) != 0 {
		t.Errorf("got %v, %v; want an empty map and no error", tags, err)
	}
}
