package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store/storetest"
	"github.com/infrashift/chit/internal/websocket"
)

// fixture is an App over the in-memory store, with one open team ("eng") and
// one open channel in it ("general"). Users are created on demand by name and
// are not members of anything until a test says so.
type fixture struct {
	t       *testing.T
	app     *App
	store   *storetest.Store
	keto    *fakeKeto
	team    *model.Team
	channel *model.Channel
	users   map[string]*model.User
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	keto := newFakeKeto(t)
	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	cfg := config.Defaults()
	cfg.KetoReadURL = keto.srv.URL
	cfg.KetoWriteURL = keto.srv.URL
	cfg.ZincSearchURL = ""

	ms := storetest.New()
	f := &fixture{
		t:     t,
		app:   New(ms, hub, nil, cfg),
		store: ms,
		keto:  keto,
		users: map[string]*model.User{},
		team: &model.Team{
			ID: model.NewID(), Name: "eng", DisplayName: "Eng", Type: model.TeamOpen, CreateAt: 1, UpdateAt: 1,
		},
	}
	f.channel = &model.Channel{
		ID: model.NewID(), TeamID: f.team.ID, Name: "general", DisplayName: "General",
		Type: model.ChannelOpen, CreateAt: 1, UpdateAt: 1,
	}
	ms.Teams.Seed(f.team)
	ms.Channels.Seed(f.channel)
	// Runs before the fake Keto server closes (cleanups run last-in first-out).
	t.Cleanup(f.app.WaitBackground)
	return f
}

// user returns the user with this name, creating it on first use.
func (f *fixture) user(name string) *model.User {
	if u, ok := f.users[name]; ok {
		return u
	}
	u := &model.User{
		ID: model.NewID(), KratosID: model.NewID(), Username: name, DisplayName: name,
		Email: name + "@example.com", Roles: "system_user", ActorType: model.ActorTypeUser,
		CreateAt: 1, UpdateAt: 1,
	}
	f.store.Users.Seed(u)
	f.users[name] = u
	return u
}

// admin returns a system_admin with this name.
func (f *fixture) admin(name string) *model.User {
	u := f.user(name)
	u.Roles = "system_user system_admin"
	f.store.Users.Seed(u)
	return u
}

// joinTeam adds the named users to the team.
func (f *fixture) joinTeam(names ...string) {
	for _, n := range names {
		f.store.Teams.SeedMember(&model.TeamMember{TeamID: f.team.ID, UserID: f.user(n).ID, Roles: model.TeamRoleUser})
	}
}

// join adds the named users to the team and to channel.
func (f *fixture) join(channel *model.Channel, names ...string) {
	f.joinTeam(names...)
	for _, n := range names {
		f.store.Channels.SeedMember(&model.ChannelMember{ChannelID: channel.ID, UserID: f.user(n).ID})
	}
}

// privateChannel seeds a private channel in the team.
func (f *fixture) privateChannel(name string) *model.Channel {
	c := &model.Channel{
		ID: model.NewID(), TeamID: f.team.ID, Name: name, DisplayName: name,
		Type: model.ChannelPrivate, CreateAt: 1, UpdateAt: 1,
	}
	f.store.Channels.Seed(c)
	return c
}

// post seeds a post by the named user.
func (f *fixture) post(channel *model.Channel, author, content string, createAt int64) *model.Post {
	p := &model.Post{
		ID: model.NewID(), ChannelID: channel.ID, UserID: f.user(author).ID,
		Content: content, CreateAt: createAt, UpdateAt: createAt, Props: map[string]any{},
	}
	f.store.Posts.Seed(p)
	return p
}

// fakeKeto answers permission checks with allowed and records relation writes.
type fakeKeto struct {
	srv     *httptest.Server
	mu      sync.Mutex
	allowed bool
	writes  int
}

func newFakeKeto(t *testing.T) *fakeKeto {
	k := &fakeKeto{allowed: true}
	k.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k.mu.Lock()
		defer k.mu.Unlock()
		if r.URL.Path == "/admin/relation-tuples" {
			k.writes++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"allowed": k.allowed})
	}))
	t.Cleanup(k.srv.Close)
	return k
}

func (k *fakeKeto) setAllowed(v bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.allowed = v
}
