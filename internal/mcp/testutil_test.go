package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// ─── Pre-seeded test IDs ─────────────────────────────────────────

const (
	agentUserID = "agent-user-001"
	extraUserID = "user-002"
	teamID      = "team-001"
	channelID   = "channel-001"
	rootPostID  = "post-001"
	replyPostID = "post-002"
	tagID       = "tag-001"
)

// ─── noopPubSub ──────────────────────────────────────────────────

type noopPubSub struct{}

func (noopPubSub) Publish(_ context.Context, _ string, _ []byte) error              { return nil }
func (noopPubSub) Subscribe(_ context.Context, _ string, _ func(data []byte)) error { return nil }
func (noopPubSub) Close() error                                                     { return nil }

// ─── Mock Store ──────────────────────────────────────────────────

type mockStore struct {
	user    *mockUserStore
	team    *mockTeamStore
	channel *mockChannelStore
	post    *mockPostStore
	thread  *mockThreadStore
	tag     *mockTagStore
}

func (m *mockStore) User() store.UserStore       { return m.user }
func (m *mockStore) Team() store.TeamStore       { return m.team }
func (m *mockStore) Channel() store.ChannelStore { return m.channel }
func (m *mockStore) Post() store.PostStore       { return m.post }
func (m *mockStore) Thread() store.ThreadStore   { return m.thread }
func (m *mockStore) Tag() store.TagStore         { return m.tag }
func (m *mockStore) Close()                      {}

// ─── Mock UserStore ──────────────────────────────────────────────

type mockUserStore struct {
	mu    sync.RWMutex
	byID  map[string]*model.User
	byUN  map[string]*model.User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		byID: make(map[string]*model.User),
		byUN: make(map[string]*model.User),
	}
}

func (s *mockUserStore) seed(u *model.User) {
	s.byID[u.ID] = u
	s.byUN[u.Username] = u
}

func (s *mockUserStore) Save(_ context.Context, u *model.User) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[u.ID] = u
	s.byUN[u.Username] = u
	return u, nil
}

func (s *mockUserStore) Get(_ context.Context, id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	cp := *u
	return &cp, nil
}

func (s *mockUserStore) GetByKratosID(_ context.Context, _ string) (*model.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockUserStore) GetByUsername(_ context.Context, username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byUN[username]
	if !ok {
		return nil, fmt.Errorf("user %s not found", username)
	}
	cp := *u
	return &cp, nil
}

func (s *mockUserStore) GetByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockUserStore) Update(_ context.Context, u *model.User) (*model.User, error) {
	return s.Save(context.Background(), u)
}

func (s *mockUserStore) Search(_ context.Context, _ string, _, _ int) ([]*model.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockUserStore) GetByIDs(_ context.Context, _ []string) ([]*model.User, error) {
	return nil, fmt.Errorf("not implemented")
}

// ─── Mock TeamStore ──────────────────────────────────────────────

type mockTeamStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.Team
	members map[string][]*model.TeamMember // teamID -> members
}

func newMockTeamStore() *mockTeamStore {
	return &mockTeamStore{
		byID:    make(map[string]*model.Team),
		members: make(map[string][]*model.TeamMember),
	}
}

func (s *mockTeamStore) seed(t *model.Team) { s.byID[t.ID] = t }

func (s *mockTeamStore) Save(_ context.Context, t *model.Team) (*model.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[t.ID] = t
	return t, nil
}

func (s *mockTeamStore) Get(_ context.Context, id string) (*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("team %s not found", id)
	}
	return t, nil
}

func (s *mockTeamStore) GetByName(_ context.Context, _ string) (*model.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockTeamStore) Update(_ context.Context, t *model.Team) (*model.Team, error) {
	return s.Save(context.Background(), t)
}

func (s *mockTeamStore) Delete(_ context.Context, _ string, _ int64) error {
	return fmt.Errorf("not implemented")
}

func (s *mockTeamStore) GetAll(_ context.Context, _, _ int) ([]*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var teams []*model.Team
	for _, t := range s.byID {
		teams = append(teams, t)
	}
	return teams, nil
}

func (s *mockTeamStore) GetTeamsForUser(_ context.Context, _ string) ([]*model.Team, error) {
	return s.GetAll(context.Background(), 0, 100)
}

func (s *mockTeamStore) SaveMember(_ context.Context, m *model.TeamMember) (*model.TeamMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.TeamID] = append(s.members[m.TeamID], m)
	return m, nil
}

func (s *mockTeamStore) RemoveMember(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

func (s *mockTeamStore) GetMembers(_ context.Context, teamID string, _, _ int) ([]*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.members[teamID], nil
}

func (s *mockTeamStore) GetMember(_ context.Context, teamID, userID string) (*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.members[teamID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, model.NewNotFoundError("mockTeamStore.GetMember", teamID+"/"+userID)
}

// ─── Mock ChannelStore ───────────────────────────────────────────

type mockChannelStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.Channel
	members map[string][]*model.ChannelMember // channelID -> members
}

func newMockChannelStore() *mockChannelStore {
	return &mockChannelStore{
		byID:    make(map[string]*model.Channel),
		members: make(map[string][]*model.ChannelMember),
	}
}

func (s *mockChannelStore) seed(c *model.Channel)                    { s.byID[c.ID] = c }
func (s *mockChannelStore) seedMember(m *model.ChannelMember) {
	s.members[m.ChannelID] = append(s.members[m.ChannelID], m)
}

func (s *mockChannelStore) Save(_ context.Context, c *model.Channel) (*model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = c
	return c, nil
}

func (s *mockChannelStore) Get(_ context.Context, id string) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("channel %s not found", id)
	}
	return c, nil
}

func (s *mockChannelStore) Update(_ context.Context, c *model.Channel) (*model.Channel, error) {
	return s.Save(context.Background(), c)
}

func (s *mockChannelStore) Delete(_ context.Context, _ string, _ int64) error {
	return fmt.Errorf("not implemented")
}

func (s *mockChannelStore) GetChannelsForTeam(_ context.Context, teamID string, _, _ int) ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var channels []*model.Channel
	for _, c := range s.byID {
		if c.TeamID == teamID {
			channels = append(channels, c)
		}
	}
	return channels, nil
}

func (s *mockChannelStore) GetChannelsForUser(_ context.Context, _, _ string) ([]*model.Channel, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockChannelStore) SaveMember(_ context.Context, m *model.ChannelMember) (*model.ChannelMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ChannelID] = append(s.members[m.ChannelID], m)
	return m, nil
}

func (s *mockChannelStore) RemoveMember(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

func (s *mockChannelStore) GetMembers(_ context.Context, channelID string, _, _ int) ([]*model.ChannelMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.members[channelID], nil
}

func (s *mockChannelStore) GetMember(_ context.Context, channelID, userID string) (*model.ChannelMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.members[channelID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, model.NewNotFoundError("mockChannelStore.GetMember", channelID+"/"+userID)
}

func (s *mockChannelStore) GetChannelIDsForUser(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for channelID, members := range s.members {
		for _, m := range members {
			if m.UserID == userID {
				ids = append(ids, channelID)
				break
			}
		}
	}
	return ids, nil
}

func (s *mockChannelStore) UpdateLastViewedAt(_ context.Context, _, _ string, _ int64) error {
	return nil
}

func (s *mockChannelStore) GetByName(_ context.Context, _, _ string) (*model.Channel, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockChannelStore) GetDirectChannelByName(_ context.Context, _ string) (*model.Channel, error) {
	return nil, fmt.Errorf("not found")
}

func (s *mockChannelStore) SaveDirectChannel(_ context.Context, _ *model.Channel, _ []string) (*model.Channel, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockChannelStore) GetDirectChannelsForUser(_ context.Context, _ string) ([]*model.Channel, error) {
	return nil, nil
}

func (s *mockChannelStore) IncrementMsgCount(_ context.Context, _ string, _ int64) error {
	return nil
}

func (s *mockChannelStore) IncrementMentionCount(_ context.Context, channelID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members[channelID] {
		if m.UserID == userID {
			m.MentionCount++
			return nil
		}
	}
	return nil
}

// ─── Mock PostStore ──────────────────────────────────────────────

type mockPostStore struct {
	mu   sync.RWMutex
	byID map[string]*model.Post
}

func newMockPostStore() *mockPostStore {
	return &mockPostStore{
		byID: make(map[string]*model.Post),
	}
}

func (s *mockPostStore) seed(p *model.Post) { s.byID[p.ID] = p }

func (s *mockPostStore) Save(_ context.Context, p *model.Post) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.PreSave()
	s.byID[p.ID] = p
	return p, nil
}

func (s *mockPostStore) Get(_ context.Context, id string) (*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("post %s not found", id)
	}
	return p, nil
}

func (s *mockPostStore) Update(_ context.Context, p *model.Post) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[p.ID] = p
	return p, nil
}

func (s *mockPostStore) Delete(_ context.Context, _ string, _ int64) error {
	return fmt.Errorf("not implemented")
}

func (s *mockPostStore) GetPostsForChannel(_ context.Context, channelID string, _ model.GetPostsOptions) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var posts []*model.Post
	for _, p := range s.byID {
		if p.ChannelID == channelID {
			posts = append(posts, p)
		}
	}
	return &model.PostList{Order: posts}, nil
}

func (s *mockPostStore) GetPostsForThread(_ context.Context, rootID string) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var posts []*model.Post
	// Include root post and replies
	for _, p := range s.byID {
		if p.ID == rootID || p.RootID == rootID {
			posts = append(posts, p)
		}
	}
	return &model.PostList{Order: posts}, nil
}

func (s *mockPostStore) GetPinnedPosts(_ context.Context, channelID string) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var posts []*model.Post
	for _, p := range s.byID {
		if p.ChannelID == channelID && p.IsPinned {
			posts = append(posts, p)
		}
	}
	return &model.PostList{Order: posts}, nil
}

func (s *mockPostStore) SetPinned(_ context.Context, _ string, _ bool) error {
	return fmt.Errorf("not implemented")
}
func (s *mockPostStore) GetPostsSince(_ context.Context, _ int64, _ int) ([]*model.Post, error) {
	return nil, nil
}

func (s *mockPostStore) SearchByContent(_ context.Context, channelID, query string, page, perPage int) ([]*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var posts []*model.Post
	for _, p := range s.byID {
		if channelID != "" && p.ChannelID != channelID {
			continue
		}
		if strings.Contains(strings.ToLower(p.Content), strings.ToLower(query)) {
			posts = append(posts, p)
		}
	}
	start := page * perPage
	if start >= len(posts) {
		return nil, nil
	}
	end := start + perPage
	if end > len(posts) {
		end = len(posts)
	}
	return posts[start:end], nil
}

// ─── Mock ThreadStore ────────────────────────────────────────────

type mockThreadStore struct {
	mu          sync.RWMutex
	threads     map[string]*model.Thread            // postID -> thread
	memberships map[string]*model.ThreadMembership   // postID:userID -> membership
}

func newMockThreadStore() *mockThreadStore {
	return &mockThreadStore{
		threads:     make(map[string]*model.Thread),
		memberships: make(map[string]*model.ThreadMembership),
	}
}

func (s *mockThreadStore) seed(t *model.Thread) { s.threads[t.PostID] = t }
func (s *mockThreadStore) seedMembership(m *model.ThreadMembership) {
	s.memberships[m.PostID+":"+m.UserID] = m
}

func (s *mockThreadStore) SaveOrUpdate(_ context.Context, t *model.Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.threads[t.PostID]; ok {
		existing.ReplyCount = t.ReplyCount
		existing.LastReplyAt = t.LastReplyAt
	} else {
		s.threads[t.PostID] = t
	}
	return nil
}

func (s *mockThreadStore) Get(_ context.Context, postID string) (*model.Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.threads[postID]
	if !ok {
		return nil, fmt.Errorf("thread %s not found", postID)
	}
	return t, nil
}

func (s *mockThreadStore) SaveMembership(_ context.Context, m *model.ThreadMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberships[m.PostID+":"+m.UserID] = m
	return nil
}

func (s *mockThreadStore) GetMembership(_ context.Context, postID, userID string) (*model.ThreadMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.memberships[postID+":"+userID]
	if !ok {
		return nil, fmt.Errorf("membership %s:%s not found", postID, userID)
	}
	return m, nil
}

func (s *mockThreadStore) UpdateMembership(_ context.Context, m *model.ThreadMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberships[m.PostID+":"+m.UserID] = m
	return nil
}

func (s *mockThreadStore) GetThreadsForUser(_ context.Context, _, _ string, _, _ int) (*model.UserThreadList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &model.UserThreadList{
		Threads: []*model.ThreadResponse{},
		Total:   0,
	}, nil
}

func (s *mockThreadStore) IncrementReplyCount(_ context.Context, postID string, timestamp int64, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[postID]
	if !ok {
		return fmt.Errorf("thread %s not found", postID)
	}
	t.ReplyCount++
	t.LastReplyAt = timestamp
	return nil
}

func (s *mockThreadStore) MarkAsRead(_ context.Context, _, _ string, _ int64) error {
	return nil
}

func (s *mockThreadStore) IncrementMentionCount(_ context.Context, postID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := postID + ":" + userID
	if m, ok := s.memberships[key]; ok {
		m.UnreadMentionCount++
	}
	return nil
}

// ─── Mock TagStore ───────────────────────────────────────────────

type mockTagStore struct {
	mu         sync.RWMutex
	tags       map[string]*model.Tag
	postTags   map[string]map[string]bool // messageID -> set of tagIDs
}

func newMockTagStore() *mockTagStore {
	return &mockTagStore{
		tags:     make(map[string]*model.Tag),
		postTags: make(map[string]map[string]bool),
	}
}

func (s *mockTagStore) seed(t *model.Tag) { s.tags[t.ID] = t }
func (s *mockTagStore) seedPostTag(messageID, tagID string) {
	if s.postTags[messageID] == nil {
		s.postTags[messageID] = make(map[string]bool)
	}
	s.postTags[messageID][tagID] = true
}

func (s *mockTagStore) Save(_ context.Context, t *model.Tag) (*model.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[t.ID] = t
	return t, nil
}

func (s *mockTagStore) GetAll(_ context.Context) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var tags []*model.Tag
	for _, t := range s.tags {
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *mockTagStore) AddTagToPost(_ context.Context, messageID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.postTags[messageID] == nil {
		s.postTags[messageID] = make(map[string]bool)
	}
	s.postTags[messageID][tagID] = true
	return nil
}

func (s *mockTagStore) RemoveTagFromPost(_ context.Context, messageID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.postTags[messageID]; ok {
		delete(m, tagID)
	}
	return nil
}

func (s *mockTagStore) GetTagsForPost(_ context.Context, messageID string) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var tags []*model.Tag
	if tagIDs, ok := s.postTags[messageID]; ok {
		for tid := range tagIDs {
			if t, exists := s.tags[tid]; exists {
				tags = append(tags, t)
			}
		}
	}
	return tags, nil
}

func (s *mockTagStore) GetPostIDsByTags(_ context.Context, tagIDs []string, page, perPage int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []string
	for msgID, tags := range s.postTags {
		matchCount := 0
		for _, tid := range tagIDs {
			if tags[tid] {
				matchCount++
			}
		}
		if matchCount == len(tagIDs) {
			result = append(result, msgID)
		}
	}
	start := page * perPage
	if start >= len(result) {
		return nil, nil
	}
	end := start + perPage
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}

func (s *mockTagStore) FilterPostIDsByTags(_ context.Context, postIDs []string, tagIDs []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	postSet := make(map[string]bool, len(postIDs))
	for _, id := range postIDs {
		postSet[id] = true
	}
	var result []string
	for msgID, tags := range s.postTags {
		if !postSet[msgID] {
			continue
		}
		matchCount := 0
		for _, tid := range tagIDs {
			if tags[tid] {
				matchCount++
			}
		}
		if matchCount == len(tagIDs) {
			result = append(result, msgID)
		}
	}
	return result, nil
}

// ─── Setup helper ────────────────────────────────────────────────

// setupTestMCP creates a ChitMCPServer backed by mock stores and connects
// an MCP client to it via in-memory transport.
// Returns (ctx, clientSession, mcpServer, cleanup).
func setupTestMCP(t *testing.T) (context.Context, *mcpsdk.ClientSession, *ChitMCPServer, func()) {
	t.Helper()

	// Build mock store with pre-seeded data
	ms := &mockStore{
		user:    newMockUserStore(),
		team:    newMockTeamStore(),
		channel: newMockChannelStore(),
		post:    newMockPostStore(),
		thread:  newMockThreadStore(),
		tag:     newMockTagStore(),
	}

	ms.user.seed(&model.User{
		ID:          agentUserID,
		KratosID:    "kratos-agent",
		Username:    "agent-bot",
		DisplayName: "Agent Bot",
		Email:       "agent@chit.local",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.user.seed(&model.User{
		ID:          extraUserID,
		KratosID:    "kratos-alice",
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@chit.local",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	})

	ms.team.seed(&model.Team{
		ID:          teamID,
		Name:        "engineering",
		DisplayName: "Engineering",
		Type:        "O",
		CreatorID:   extraUserID,
		CreateAt:    1000,
		UpdateAt:    1000,
	})

	_, _ = ms.team.SaveMember(context.Background(), &model.TeamMember{
		TeamID: teamID,
		UserID: agentUserID,
	})

	ms.channel.seed(&model.Channel{
		ID:          channelID,
		TeamID:      teamID,
		CreatorID:   extraUserID,
		Name:        "general",
		DisplayName: "General",
		Type:        "O",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.channel.seedMember(&model.ChannelMember{
		ChannelID: channelID,
		UserID:    agentUserID,
		CreateAt:  1000,
	})

	ms.post.seed(&model.Post{
		ID:        rootPostID,
		ChannelID: channelID,
		UserID:    extraUserID,
		Content:   "Hello world",
		CreateAt:  2000,
		UpdateAt:  2000,
	})
	ms.post.seed(&model.Post{
		ID:        replyPostID,
		ChannelID: channelID,
		UserID:    agentUserID,
		RootID:    rootPostID,
		Content:   "Hi there",
		CreateAt:  3000,
		UpdateAt:  3000,
	})

	ms.thread.seed(&model.Thread{
		PostID:      rootPostID,
		ChannelID:   channelID,
		ReplyCount:  1,
		LastReplyAt: 3000,
	})
	ms.thread.seedMembership(&model.ThreadMembership{
		PostID:    rootPostID,
		UserID:    agentUserID,
		Following: true,
	})

	ms.tag.seed(&model.Tag{ID: tagID, Name: "important"})
	ms.tag.seedPostTag(rootPostID, tagID)

	// Build App with real Hub (in-memory, no external deps) and noop pubsub
	cfg := config.Defaults()
	cfg.ZincSearchURL = "" // Use SQL fallback instead of ZincSearch
	hub := websocket.NewHub(nil)
	a := app.New(ms, hub, noopPubSub{}, cfg)

	// Create MCP server
	mcpSrv := New(a, agentUserID)

	// Connect via in-memory transport
	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	// Server must connect first
	_, err := mcpSrv.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	// Client connects
	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "test-client",
		Version: "0.0.1",
	}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	cleanup := func() {
		cs.Close()
		hub.Stop()
	}

	return ctx, cs, mcpSrv, cleanup
}
