package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/chitclient"
	"github.com/infrashift/chit/internal/model"
)

// ─── Pre-seeded test IDs ─────────────────────────────────────────

const (
	agentKratosID = "kratos-agent"
	agentUserID   = "agent-user-001"
	extraUserID   = "user-002"
	teamID        = "team-001"
	channelID     = "channel-001"
	rootPostID    = "post-001"
	replyPostID   = "post-002"
	tagID         = "tag-001"
)

// ─── Fake chitd ──────────────────────────────────────────────────

// fakeChit is an httptest-backed stand-in for chitd's REST API and WebSocket,
// pre-seeded with the same fixtures the MCP tests have always used.
type fakeChit struct {
	server *httptest.Server
	// wsCh delivers events to any connected WebSocket client.
	wsCh chan *model.WebSocketEvent

	mu      sync.Mutex
	agent   model.User
	alice   model.User
	team    model.Team
	channel model.Channel
	root    model.Post
	reply   model.Post
	tag     model.Tag
	postSeq int
}

func newFakeChit(t *testing.T) *fakeChit {
	t.Helper()

	f := &fakeChit{
		wsCh: make(chan *model.WebSocketEvent, 16),
		agent: model.User{
			ID: agentUserID, KratosID: agentKratosID, Username: "agent-bot",
			DisplayName: "Agent Bot", Email: "agent@chit.local",
			ActorType: model.ActorTypeAgent, Roles: "system_user",
			CreateAt: 1000, UpdateAt: 1000,
		},
		alice: model.User{
			ID: extraUserID, KratosID: "kratos-alice", Username: "alice",
			DisplayName: "Alice", Email: "alice@chit.local",
			Roles: "system_user", CreateAt: 1000, UpdateAt: 1000,
		},
		team: model.Team{
			ID: teamID, Name: "engineering", DisplayName: "Engineering",
			Type: "O", CreatorID: extraUserID, CreateAt: 1000, UpdateAt: 1000,
		},
		channel: model.Channel{
			ID: channelID, TeamID: teamID, CreatorID: extraUserID,
			Name: "general", DisplayName: "General", Type: "O",
			CreateAt: 1000, UpdateAt: 1000,
		},
		root: model.Post{
			ID: rootPostID, ChannelID: channelID, UserID: extraUserID,
			Content: "Hello world", CreateAt: 2000, UpdateAt: 2000,
		},
		reply: model.Post{
			ID: replyPostID, ChannelID: channelID, UserID: agentUserID,
			RootID: rootPostID, Content: "Hi there", CreateAt: 3000, UpdateAt: 3000,
		},
		tag: model.Tag{ID: tagID, Name: "important"},
	}

	r := chi.NewRouter()

	// Every request must carry the agent's Kratos ID, exactly like chitd's
	// trusted-proxy-header auth.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("X-User-Id") != agentKratosID {
				http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, req)
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/users/me", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, f.agent) // own profile keeps its email
		})
		r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
			u, ok := f.userByID(chi.URLParam(req, "id"))
			if !ok {
				http.NotFound(w, req)
				return
			}
			writeJSON(w, sanitized(&u))
		})
		r.Get("/users/username/{username}", func(w http.ResponseWriter, req *http.Request) {
			u, ok := f.userByUsername(chi.URLParam(req, "username"))
			if !ok {
				http.NotFound(w, req)
				return
			}
			writeJSON(w, sanitized(&u))
		})

		r.Get("/teams", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, []*model.Team{&f.team})
		})
		r.Get("/teams/{id}", func(w http.ResponseWriter, req *http.Request) {
			if chi.URLParam(req, "id") != teamID {
				http.NotFound(w, req)
				return
			}
			writeJSON(w, f.team)
		})
		r.Get("/teams/{id}/channels", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, []*model.Channel{&f.channel})
		})

		r.Get("/channels/{id}", func(w http.ResponseWriter, req *http.Request) {
			if chi.URLParam(req, "id") != channelID {
				http.NotFound(w, req)
				return
			}
			writeJSON(w, f.channel)
		})
		r.Get("/channels/{id}/members", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, []*model.ChannelMember{
				{ChannelID: channelID, UserID: agentUserID, CreateAt: 1000},
			})
		})
		r.Get("/channels/{id}/posts", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, &model.PostList{Order: []*model.Post{&f.reply, &f.root}})
		})
		r.Get("/channels/{id}/pinned", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, &model.PostList{Order: []*model.Post{}})
		})
		r.Post("/channels/{id}/members/me/view", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, map[string]string{"status": "OK"})
		})

		r.Post("/posts", func(w http.ResponseWriter, req *http.Request) {
			var post model.Post
			if err := json.NewDecoder(req.Body).Decode(&post); err != nil {
				http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
				return
			}
			f.mu.Lock()
			f.postSeq++
			post.ID = fmt.Sprintf("post-new-%03d", f.postSeq)
			f.mu.Unlock()
			post.UserID = agentUserID // chitd stamps the authenticated user
			writeJSON(w, post)
		})
		r.Get("/posts/{id}", func(w http.ResponseWriter, req *http.Request) {
			p, ok := f.postByID(chi.URLParam(req, "id"))
			if !ok {
				http.NotFound(w, req)
				return
			}
			writeJSON(w, p)
		})
		r.Get("/posts/{id}/thread", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, &model.PostList{Order: []*model.Post{&f.root, &f.reply}})
		})
		r.Get("/posts/{id}/tags", func(w http.ResponseWriter, req *http.Request) {
			if chi.URLParam(req, "id") == rootPostID {
				writeJSON(w, []*model.Tag{&f.tag})
				return
			}
			writeJSON(w, []*model.Tag{})
		})
		r.Post("/posts/{id}/tags", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				TagID string `json:"tag_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.TagID == "" {
				http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"status": "OK"})
		})
		r.Post("/posts/search", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Terms  string   `json:"terms"`
				TagIDs []string `json:"tag_ids"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
				return
			}
			if strings.Contains(f.root.Content, body.Terms) && body.Terms != "" {
				writeJSON(w, &model.PostList{Order: []*model.Post{&f.root}})
				return
			}
			writeJSON(w, &model.PostList{Order: []*model.Post{}})
		})

		r.Get("/tags", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, []*model.Tag{&f.tag})
		})

		r.Get("/users/me/teams/{id}/threads", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, &model.UserThreadList{Threads: []*model.ThreadResponse{}, Total: 0})
		})
		r.Put("/users/me/teams/{team_id}/threads/{id}/read", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, map[string]string{"status": "OK"})
		})
		r.Put("/users/me/teams/{team_id}/threads/{id}/following", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Following bool `json:"following"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"status": "OK"})
		})

		r.Get("/websocket", f.handleWS)
	})

	f.server = httptest.NewServer(r)
	t.Cleanup(f.close)
	return f
}

func (f *fakeChit) close() {
	f.mu.Lock()
	if f.wsCh != nil {
		close(f.wsCh)
		f.wsCh = nil
	}
	f.mu.Unlock()
	f.server.Close()
}

// pushEvent delivers an event to connected WebSocket clients.
func (f *fakeChit) pushEvent(ev *model.WebSocketEvent) {
	f.mu.Lock()
	ch := f.wsCh
	f.mu.Unlock()
	if ch != nil {
		ch <- ev
	}
}

var upgrader = websocket.Upgrader{}

func (f *fakeChit) handleWS(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	f.mu.Lock()
	ch := f.wsCh
	f.mu.Unlock()
	if ch == nil {
		return
	}
	for ev := range ch {
		if err := conn.WriteJSON(ev); err != nil {
			return
		}
	}
}

func (f *fakeChit) userByID(id string) (model.User, bool) {
	switch id {
	case agentUserID:
		return f.agent, true
	case extraUserID:
		return f.alice, true
	}
	return model.User{}, false
}

func (f *fakeChit) userByUsername(username string) (model.User, bool) {
	switch username {
	case f.agent.Username:
		return f.agent, true
	case f.alice.Username:
		return f.alice, true
	}
	return model.User{}, false
}

func (f *fakeChit) postByID(id string) (model.Post, bool) {
	switch id {
	case rootPostID:
		return f.root, true
	case replyPostID:
		return f.reply, true
	}
	return model.Post{}, false
}

func sanitized(u *model.User) *model.User {
	cp := *u
	cp.Sanitize()
	return &cp
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ─── Setup helper ────────────────────────────────────────────────

// setupTestMCP creates a ChitMCPServer backed by a fake chitd HTTP server and
// connects an MCP client to it via in-memory transport.
// Returns (ctx, clientSession, mcpServer, cleanup).
func setupTestMCP(t *testing.T) (context.Context, *mcpsdk.ClientSession, *ChitMCPServer, func()) {
	t.Helper()

	fake := newFakeChit(t)
	client := chitclient.New(fake.server.URL, agentKratosID, "")
	mcpSrv := New(client)

	// Connect via in-memory transport
	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	// Server must connect first
	_, err := mcpSrv.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	// Client connects
	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "test-client",
		Version: "0.0.1",
	}, nil)
	cs, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	cleanup := func() {
		_ = cs.Close()
	}

	return ctx, cs, mcpSrv, cleanup
}
