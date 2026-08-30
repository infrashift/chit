package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// ─── Minimal mocks for command tests ─────────────────────────────

type cmdMockUserStore struct{ mentionMockUserStore }

type cmdMockStore struct {
	user    *cmdMockUserStore
	team    mentionMockTeamStore
	channel *mentionMockChannelStore
	post    *cmdMockPostStore
	thread  *mentionMockThreadStore
	tag     mentionMockTagStore
}

func (m *cmdMockStore) User() store.UserStore       { return m.user }
func (m *cmdMockStore) Team() store.TeamStore       { return m.team }
func (m *cmdMockStore) Channel() store.ChannelStore { return m.channel }
func (m *cmdMockStore) Post() store.PostStore       { return m.post }
func (m *cmdMockStore) Thread() store.ThreadStore   { return m.thread }
func (m *cmdMockStore) Tag() store.TagStore         { return m.tag }
func (m *cmdMockStore) Close()                      {}

type cmdMockPostStore struct {
	saved []*model.Post
}

func (s *cmdMockPostStore) Save(_ context.Context, p *model.Post) (*model.Post, error) {
	p.PreSave()
	s.saved = append(s.saved, p)
	return p, nil
}
func (s *cmdMockPostStore) Get(_ context.Context, _ string) (*model.Post, error) {
	return nil, errNotFound
}
func (s *cmdMockPostStore) Update(_ context.Context, p *model.Post) (*model.Post, error) {
	return p, nil
}
func (s *cmdMockPostStore) Delete(_ context.Context, _ string, _ int64) error { return nil }
func (s *cmdMockPostStore) GetPostsForChannel(_ context.Context, _ string, _ model.GetPostsOptions) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *cmdMockPostStore) GetPostsForThread(_ context.Context, _ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *cmdMockPostStore) GetPinnedPosts(_ context.Context, _ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *cmdMockPostStore) SetPinned(_ context.Context, _ string, _ bool) error { return nil }
func (s *cmdMockPostStore) SearchByContent(_ context.Context, _, _ string, _, _ int) ([]*model.Post, error) {
	return nil, nil
}
func (s *cmdMockPostStore) GetPostsSince(_ context.Context, _ int64, _ int) ([]*model.Post, error) {
	return nil, nil
}

// ─── Test helpers ────────────────────────────────────────────────

func newCommandTestApp(t *testing.T, ketoAllowed bool) *App {
	t.Helper()

	// Fake Keto server returning allowed/denied.
	ketoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"allowed": ketoAllowed})
	}))
	t.Cleanup(ketoSrv.Close)

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	ms := &cmdMockStore{
		user: &cmdMockUserStore{mentionMockUserStore{
			users: map[string]*model.User{
				"testuser": {ID: "user-001", Username: "testuser"},
			},
		}},
		channel: &mentionMockChannelStore{
			members: map[string][]*model.ChannelMember{
				"ch-001": {{ChannelID: "ch-001", UserID: "user-001"}},
			},
		},
		post:   &cmdMockPostStore{},
		thread: &mentionMockThreadStore{memberships: make(map[string]*model.ThreadMembership)},
	}

	cfg := config.Defaults()
	cfg.KetoReadURL = ketoSrv.URL

	a := New(ms, hub, nil, cfg)

	// Set up command registry with two commands.
	cmds := []*command.Command{
		{ID: "help", Slug: "help", Description: "List available commands", Category: "chat"},
		{ID: "kick", Slug: "kick", Description: "Remove a user", Category: "admin"},
	}
	a.CommandRegistry = command.NewRegistry(cmds)
	a.CommandHandlers = map[string]command.Handler{
		"help": command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
			return &command.CommandResult{ResponseText: "Available commands: /help, /kick"}, nil
		}),
		"kick": command.HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*command.CommandResult, error) {
			return &command.CommandResult{ResponseText: fmt.Sprintf("Kicked %s", args), Triggered: true}, nil
		}),
	}

	// Audit logger to temp file.
	al, err := command.NewAuditLogger("-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { al.Close() })
	a.AuditLogger = al

	return a
}

// ─── Tests ───────────────────────────────────────────────────────

func TestInterceptSlashCommand_NotACommand(t *testing.T) {
	a := newCommandTestApp(t, true)
	resp, handled, err := a.InterceptSlashCommand(context.Background(), "user-001", "ch-001", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for normal message")
	}
	if resp != nil {
		t.Fatal("expected nil response for normal message")
	}
}

func TestInterceptSlashCommand_UnknownCommand(t *testing.T) {
	a := newCommandTestApp(t, true)
	resp, handled, err := a.InterceptSlashCommand(context.Background(), "user-001", "ch-001", "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true for unknown command")
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected non-empty error response")
	}
}

func TestInterceptSlashCommand_Authorized(t *testing.T) {
	a := newCommandTestApp(t, true)
	resp, handled, err := a.InterceptSlashCommand(context.Background(), "user-001", "ch-001", "/help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected non-empty response")
	}
	if resp.Text != "Available commands: /help, /kick" {
		t.Errorf("unexpected response text: %s", resp.Text)
	}
}

func TestInterceptSlashCommand_Denied(t *testing.T) {
	a := newCommandTestApp(t, false)
	resp, handled, err := a.InterceptSlashCommand(context.Background(), "user-001", "ch-001", "/help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true for denied command")
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected non-empty denial response")
	}
}

func TestInterceptSlashCommand_WebhookTriggered(t *testing.T) {
	a := newCommandTestApp(t, true)
	ch := make(chan *command.WebhookEvent, 10)
	a.WebhookCh = ch

	_, handled, err := a.InterceptSlashCommand(context.Background(), "user-001", "ch-001", "/kick alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}

	select {
	case evt := <-ch:
		if evt.CommandSlug != "kick" {
			t.Errorf("expected slug=kick, got %s", evt.CommandSlug)
		}
		if evt.Args != "alice" {
			t.Errorf("expected args=alice, got %s", evt.Args)
		}
		if evt.Status != "executed" {
			t.Errorf("expected status=executed, got %s", evt.Status)
		}
	default:
		t.Fatal("expected webhook event on channel")
	}
}

func TestCreatePost_SlashCommandNotPersisted(t *testing.T) {
	a := newCommandTestApp(t, true)
	ms := a.Store.(*cmdMockStore)

	post := &model.Post{
		ChannelID: "ch-001",
		UserID:    "user-001",
		Content:   "/help",
		CreateAt:  model.GetMillis(),
	}

	result, err := a.CreatePost(context.Background(), post)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result should be an ephemeral command response.
	if result.Type != "command_response" {
		t.Errorf("expected type=command_response, got %s", result.Type)
	}
	ephemeral, _ := result.Props["ephemeral"].(bool)
	if !ephemeral {
		t.Error("expected props.ephemeral=true")
	}

	// The post should NOT have been saved to the store.
	if len(ms.post.saved) != 0 {
		t.Errorf("expected 0 saved posts, got %d", len(ms.post.saved))
	}
}

func TestCreatePost_NormalPostStillPersisted(t *testing.T) {
	a := newCommandTestApp(t, true)
	ms := a.Store.(*cmdMockStore)

	post := &model.Post{
		ChannelID: "ch-001",
		UserID:    "user-001",
		Content:   "hello world",
		CreateAt:  model.GetMillis(),
	}

	result, err := a.CreatePost(context.Background(), post)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Normal posts should be persisted.
	if result.Type == "command_response" {
		t.Error("expected normal post, got command_response")
	}
	if len(ms.post.saved) != 1 {
		t.Errorf("expected 1 saved post, got %d", len(ms.post.saved))
	}
}
