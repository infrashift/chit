package app

import (
	"context"
	"testing"

	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// ─── Search-specific mock stores ────────────────────────────────

type searchMockPostStore struct {
	searchResults []*model.Post
	getResults    map[string]*model.Post
}

func (s *searchMockPostStore) Save(_ context.Context, p *model.Post) (*model.Post, error) {
	return p, nil
}
func (s *searchMockPostStore) Get(_ context.Context, id string) (*model.Post, error) {
	if p, ok := s.getResults[id]; ok {
		return p, nil
	}
	return nil, errNotFound
}
func (s *searchMockPostStore) Update(_ context.Context, p *model.Post) (*model.Post, error) {
	return p, nil
}
func (s *searchMockPostStore) Delete(_ context.Context, _ string, _ int64) error { return nil }
func (s *searchMockPostStore) GetPostsForChannel(_ context.Context, _ string, _ model.GetPostsOptions) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *searchMockPostStore) GetPostsForThread(_ context.Context, _ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *searchMockPostStore) GetPinnedPosts(_ context.Context, _ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (s *searchMockPostStore) SetPinned(_ context.Context, _ string, _ bool) error { return nil }
func (s *searchMockPostStore) GetPostsSince(_ context.Context, _ int64, _ int) ([]*model.Post, error) {
	return nil, nil
}
func (s *searchMockPostStore) SearchByContent(_ context.Context, _, _ string, _, _ int) ([]*model.Post, error) {
	return s.searchResults, nil
}

type searchMockTagStore struct {
	postIDsByTags    []string
	filterPostIDsRes []string
}

func (s searchMockTagStore) Save(_ context.Context, _ *model.Tag) (*model.Tag, error) {
	return nil, nil
}
func (s searchMockTagStore) GetAll(_ context.Context) ([]*model.Tag, error)         { return nil, nil }
func (s searchMockTagStore) AddTagToPost(_ context.Context, _, _ string) error      { return nil }
func (s searchMockTagStore) RemoveTagFromPost(_ context.Context, _, _ string) error { return nil }
func (s searchMockTagStore) GetTagsForPost(_ context.Context, _ string) ([]*model.Tag, error) {
	return nil, nil
}
func (s searchMockTagStore) GetTagsForPosts(_ context.Context, _ []string) (map[string][]*model.Tag, error) {
	return map[string][]*model.Tag{}, nil
}
func (s searchMockTagStore) GetPostIDsByTags(_ context.Context, _ []string, _, _ int) ([]string, error) {
	return s.postIDsByTags, nil
}
func (s searchMockTagStore) FilterPostIDsByTags(_ context.Context, _ []string, _ []string) ([]string, error) {
	return s.filterPostIDsRes, nil
}

type searchMockStore struct {
	post    *searchMockPostStore
	tag     searchMockTagStore
	channel *mentionMockChannelStore
}

func (m *searchMockStore) User() store.UserStore { return &mentionMockUserStore{} }
func (m *searchMockStore) Team() store.TeamStore { return mentionMockTeamStore{} }
func (m *searchMockStore) Channel() store.ChannelStore {
	if m.channel == nil {
		m.channel = &mentionMockChannelStore{}
	}
	return m.channel
}
func (m *searchMockStore) Post() store.PostStore     { return m.post }
func (m *searchMockStore) Thread() store.ThreadStore { return &mentionMockThreadStore{} }
func (m *searchMockStore) Tag() store.TagStore       { return m.tag }
func (m *searchMockStore) Close()                    {}

func newSearchTestApp(t *testing.T, posts []*model.Post, zincURL string) *App {
	t.Helper()

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	getResults := make(map[string]*model.Post)
	for _, p := range posts {
		getResults[p.ID] = p
	}

	ms := &searchMockStore{
		post: &searchMockPostStore{
			searchResults: posts,
			getResults:    getResults,
		},
		channel: newSearchMockChannelStore(),
	}

	return &App{
		Store: ms,
		Hub:   hub,
		Config: &config.Config{
			ZincSearchURL: zincURL,
		},
	}
}

const searcherID = "user-searcher"

// newSearchMockChannelStore seeds ch1 with the searcher as a member so
// SearchPosts membership checks pass.
func newSearchMockChannelStore() *mentionMockChannelStore {
	return &mentionMockChannelStore{
		members: map[string][]*model.ChannelMember{
			"ch1": {{ChannelID: "ch1", UserID: searcherID}},
		},
	}
}

// ─── Tests ──────────────────────────────────────────────────────

func TestSearchPosts_ZincEmptyFallsBackToSQL(t *testing.T) {
	// ZincSearch is configured but unreachable (empty URL disables it, but
	// a non-empty URL that returns empty results should fall back to SQL).
	// We simulate this by using a ZincSearch URL that will fail to connect,
	// which makes zincSearch return (nil, false, nil) → SQL fallback.
	posts := []*model.Post{
		{ID: "p1", ChannelID: "ch1", Content: "hello world"},
		{ID: "p2", ChannelID: "ch1", Content: "hello again"},
	}

	// Empty ZincSearchURL means zincSearch returns (nil, false, nil) → SQL fallback
	a := newSearchTestApp(t, posts, "")

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "hello", nil, 0, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Order) != 2 {
		t.Errorf("expected 2 posts from SQL fallback, got %d", len(result.Order))
	}
}

func TestSearchPosts_TextOnlySQLFallback(t *testing.T) {
	// When ZincSearch URL is empty (disabled), SQL fallback is used directly.
	posts := []*model.Post{
		{ID: "p1", ChannelID: "ch1", Content: "test content"},
	}
	a := newSearchTestApp(t, posts, "")

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "test", nil, 0, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Order) != 1 {
		t.Errorf("expected 1 post, got %d", len(result.Order))
	}
	if result.Order[0].ID != "p1" {
		t.Errorf("expected post p1, got %s", result.Order[0].ID)
	}
}

func TestSearchPosts_NoQueryNoTags_ReturnsEmpty(t *testing.T) {
	a := newSearchTestApp(t, nil, "")

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "", nil, 0, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Order) != 0 {
		t.Errorf("expected 0 posts, got %d", len(result.Order))
	}
}

func TestSearchPosts_TagOnly(t *testing.T) {
	posts := []*model.Post{
		{ID: "p1", ChannelID: "ch1", Content: "tagged post"},
	}

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	ms := &searchMockStore{
		post: &searchMockPostStore{
			getResults: map[string]*model.Post{"p1": posts[0]},
		},
		tag: searchMockTagStore{
			postIDsByTags: []string{"p1"},
		},
		channel: newSearchMockChannelStore(),
	}

	a := &App{
		Store:  ms,
		Hub:    hub,
		Config: &config.Config{},
	}

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "", []string{"tag1"}, 0, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Order) != 1 {
		t.Errorf("expected 1 post, got %d", len(result.Order))
	}
}

func TestSearchPosts_HybridFallsBackToSQLWhenZincEmpty(t *testing.T) {
	posts := []*model.Post{
		{ID: "p1", ChannelID: "ch1", Content: "tagged search"},
	}

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	ms := &searchMockStore{
		post: &searchMockPostStore{
			searchResults: posts,
			getResults:    map[string]*model.Post{"p1": posts[0]},
		},
		tag: searchMockTagStore{
			filterPostIDsRes: []string{"p1"},
		},
		channel: newSearchMockChannelStore(),
	}

	// Empty ZincSearchURL → zincSearch returns (nil, false, nil) → SQL + tag filter
	a := &App{
		Store:  ms,
		Hub:    hub,
		Config: &config.Config{},
	}

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "tagged", []string{"tag1"}, 0, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Order) != 1 {
		t.Errorf("expected 1 post from SQL+tag fallback, got %d", len(result.Order))
	}
}

// Membership is not scope. A search "in this channel" was returning hits from
// every other channel the caller belonged to, because the index is queried
// without a channel filter and only membership was checked afterwards.
func TestSearchPosts_ScopesToTheRequestedChannel(t *testing.T) {
	posts := []*model.Post{
		{ID: "here", ChannelID: "ch1", Content: "needle in this channel"},
		{ID: "elsewhere", ChannelID: "ch2", Content: "needle in another channel"},
	}
	a := newSearchTestApp(t, posts, "")

	result, err := a.SearchPosts(context.Background(), "ch1", searcherID, "needle", nil, 0, 60)
	if err != nil {
		t.Fatalf("SearchPosts: %v", err)
	}

	for _, p := range result.Order {
		if p.ChannelID != "ch1" {
			t.Errorf("post %s from channel %s leaked into a search scoped to ch1",
				p.ID, p.ChannelID)
		}
	}
	if len(result.Order) != 1 {
		t.Errorf("got %d results, want just the one in ch1", len(result.Order))
	}
}

// An unscoped search spans every channel the caller can read — and only
// those. The fixture makes the searcher a member of ch1 only, so ch2 is
// excluded by membership rather than by scope.
func TestSearchPosts_UnscopedIsStillLimitedByMembership(t *testing.T) {
	posts := []*model.Post{
		{ID: "here", ChannelID: "ch1", Content: "needle one"},
		{ID: "elsewhere", ChannelID: "ch2", Content: "needle two"},
	}
	a := newSearchTestApp(t, posts, "")

	result, err := a.SearchPosts(context.Background(), "", searcherID, "needle", nil, 0, 60)
	if err != nil {
		t.Fatalf("SearchPosts: %v", err)
	}
	if len(result.Order) != 1 {
		t.Fatalf("got %d results, want only the readable one", len(result.Order))
	}
	if result.Order[0].ChannelID != "ch1" {
		t.Errorf("returned a post from %s, which the searcher cannot read",
			result.Order[0].ChannelID)
	}
}
