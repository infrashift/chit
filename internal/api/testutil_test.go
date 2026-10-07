package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// ─── Pre-seeded test IDs ─────────────────────────────────────────

const (
	testUserID    = "019421a0-0000-7000-8000-000000000001"
	testKratosID  = "kratos-001"
	testTeamID    = "019421a0-0000-7000-8000-000000000010"
	testChannelID = "019421a0-0000-7000-8000-000000000020"
	testRootPost  = "019421a0-0000-7000-8000-000000000030"
	testReplyPost = "019421a0-0000-7000-8000-000000000031"
	testTagID     = "019421a0-0000-7000-8000-000000000040"
	extraUserID   = "019421a0-0000-7000-8000-000000000002"
	thirdUserID   = "019421a0-0000-7000-8000-000000000003"
	adminUserID   = "019421a0-0000-7000-8000-000000000004"
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
	mu       sync.RWMutex
	byID     map[string]*model.User
	byUN     map[string]*model.User
	byKratos map[string]*model.User
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		byID:     make(map[string]*model.User),
		byUN:     make(map[string]*model.User),
		byKratos: make(map[string]*model.User),
	}
}

func (s *mockUserStore) seed(u *model.User) {
	s.byID[u.ID] = u
	s.byUN[u.Username] = u
	if u.KratosID != "" {
		s.byKratos[u.KratosID] = u
	}
}

func (s *mockUserStore) Save(_ context.Context, u *model.User) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[u.ID] = u
	s.byUN[u.Username] = u
	if u.KratosID != "" {
		s.byKratos[u.KratosID] = u
	}
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

func (s *mockUserStore) GetByKratosID(_ context.Context, kratosID string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byKratos[kratosID]
	if !ok {
		return nil, fmt.Errorf("user with kratos_id=%s not found", kratosID)
	}
	cp := *u
	return &cp, nil
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
	return nil, fmt.Errorf("not found")
}

func (s *mockUserStore) GetByOAuthClientID(_ context.Context, clientID string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.byID {
		if u.OAuthClientID != "" && u.OAuthClientID == clientID {
			cp := *u
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("oauth client %s not found", clientID)
}

func (s *mockUserStore) Update(_ context.Context, u *model.User) (*model.User, error) {
	return s.Save(context.Background(), u)
}

func (s *mockUserStore) Search(_ context.Context, _ string, _, _ int) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var users []*model.User
	for _, u := range s.byID {
		users = append(users, u)
	}
	return users, nil
}

func (s *mockUserStore) GetByIDs(_ context.Context, ids []string) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var users []*model.User
	for _, id := range ids {
		if u, ok := s.byID[id]; ok {
			users = append(users, u)
		}
	}
	return users, nil
}

// ─── Mock TeamStore ──────────────────────────────────────────────

type mockTeamStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.Team
	members map[string][]*model.TeamMember
}

func newMockTeamStore() *mockTeamStore {
	return &mockTeamStore{
		byID:    make(map[string]*model.Team),
		members: make(map[string][]*model.TeamMember),
	}
}

func (s *mockTeamStore) seed(t *model.Team) { s.byID[t.ID] = t }

func (s *mockTeamStore) seedMember(m *model.TeamMember) {
	s.members[m.TeamID] = append(s.members[m.TeamID], m)
}

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
	return nil, fmt.Errorf("not found")
}

func (s *mockTeamStore) Update(_ context.Context, t *model.Team) (*model.Team, error) {
	return s.Save(context.Background(), t)
}

func (s *mockTeamStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.byID[id]; ok {
		t.DeleteAt = deleteAt
	}
	return nil
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

func (s *mockTeamStore) RemoveMember(_ context.Context, teamID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	members := s.members[teamID]
	for i, m := range members {
		if m.UserID == userID {
			s.members[teamID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}
	return nil
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
	members map[string][]*model.ChannelMember
}

func newMockChannelStore() *mockChannelStore {
	return &mockChannelStore{
		byID:    make(map[string]*model.Channel),
		members: make(map[string][]*model.ChannelMember),
	}
}

func (s *mockChannelStore) seed(c *model.Channel) { s.byID[c.ID] = c }
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

func (s *mockChannelStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.byID[id]; ok {
		c.DeleteAt = deleteAt
	}
	return nil
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	var channels []*model.Channel
	for _, c := range s.byID {
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *mockChannelStore) SaveMember(_ context.Context, m *model.ChannelMember) (*model.ChannelMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ChannelID] = append(s.members[m.ChannelID], m)
	return m, nil
}

func (s *mockChannelStore) RemoveMember(_ context.Context, channelID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	members := s.members[channelID]
	for i, m := range members {
		if m.UserID == userID {
			s.members[channelID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}
	return nil
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
	return nil, fmt.Errorf("not found")
}

func (s *mockChannelStore) GetDirectChannelByName(_ context.Context, name string) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		if c.TeamID == "" && c.Name == name && c.DeleteAt == 0 {
			return c, nil
		}
	}
	return nil, fmt.Errorf("direct channel %s not found", name)
}

func (s *mockChannelStore) SaveDirectChannel(_ context.Context, c *model.Channel, userIDs []string) (*model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = model.NewID()
	}
	s.byID[c.ID] = c
	for _, uid := range userIDs {
		s.members[c.ID] = append(s.members[c.ID], &model.ChannelMember{
			ChannelID: c.ID,
			UserID:    uid,
		})
	}
	return c, nil
}

func (s *mockChannelStore) GetDirectChannelsForUser(_ context.Context, userID string) ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var channels []*model.Channel
	for _, c := range s.byID {
		if c.TeamID == "" && (c.Type == "D" || c.Type == "G") && c.DeleteAt == 0 {
			// Check if user is a member
			for _, m := range s.members[c.ID] {
				if m.UserID == userID {
					channels = append(channels, c)
					break
				}
			}
		}
	}
	return channels, nil
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
		return nil, model.NewNotFoundError("mockPostStore.Get", id)
	}
	return p, nil
}

func (s *mockPostStore) Update(_ context.Context, p *model.Post) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[p.ID] = p
	return p, nil
}

func (s *mockPostStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.byID[id]; ok {
		p.DeleteAt = deleteAt
	}
	return nil
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

func (s *mockPostStore) SetPinned(_ context.Context, id string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.byID[id]; ok {
		p.IsPinned = pinned
		return nil
	}
	return fmt.Errorf("post %s not found", id)
}
func (s *mockPostStore) GetPostsSince(_ context.Context, _ int64, _ int) ([]*model.Post, error) {
	return nil, nil
}

func (s *mockPostStore) SearchByContent(_ context.Context, _, _ string, _, _ int) ([]*model.Post, error) {
	return nil, nil
}

// ─── Mock ThreadStore ────────────────────────────────────────────

type mockThreadStore struct {
	mu          sync.RWMutex
	threads     map[string]*model.Thread
	memberships map[string]*model.ThreadMembership
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
	return &model.UserThreadList{
		Threads: []*model.ThreadResponse{},
		Total:   0,
	}, nil
}

func (s *mockThreadStore) IncrementReplyCount(_ context.Context, postID string, timestamp int64, _ string) error {
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
	mu       sync.RWMutex
	tags     map[string]*model.Tag
	postTags map[string]map[string]bool
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

func (s *mockTagStore) GetTagsForPosts(ctx context.Context, messageIDs []string) (map[string][]*model.Tag, error) {
	out := make(map[string][]*model.Tag, len(messageIDs))
	for _, id := range messageIDs {
		tags, err := s.GetTagsForPost(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(tags) > 0 {
			out[id] = tags
		}
	}
	return out, nil
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

func (s *mockTagStore) FilterPostIDsByTags(_ context.Context, postIDs, tagIDs []string) ([]string, error) {
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

func setupTestApp(t *testing.T) (*app.App, *mockStore, func()) {
	t.Helper()

	ms := &mockStore{
		user:    newMockUserStore(),
		team:    newMockTeamStore(),
		channel: newMockChannelStore(),
		post:    newMockPostStore(),
		thread:  newMockThreadStore(),
		tag:     newMockTagStore(),
	}

	ms.user.seed(&model.User{
		ID:          testUserID,
		KratosID:    testKratosID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.user.seed(&model.User{
		ID:          extraUserID,
		KratosID:    "kratos-002",
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.user.seed(&model.User{
		ID:          thirdUserID,
		KratosID:    "kratos-003",
		Username:    "charlie",
		DisplayName: "Charlie",
		Email:       "charlie@example.com",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	// The only seeded system_admin. Roles are a space-separated string, and
	// IsSystemAdmin splits on it, so "system_user system_admin" is what a real
	// admin row looks like rather than the bare role on its own.
	ms.user.seed(&model.User{
		ID:          adminUserID,
		KratosID:    "kratos-004",
		Username:    "dana",
		DisplayName: "Dana Admin",
		Email:       "dana@example.com",
		Roles:       "system_user system_admin",
		CreateAt:    1000,
		UpdateAt:    1000,
	})

	ms.team.seed(&model.Team{
		ID:          testTeamID,
		Name:        "engineering",
		DisplayName: "Engineering",
		Type:        "O",
		CreatorID:   testUserID,
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.team.seedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: testUserID,
	})
	ms.team.seedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: extraUserID,
	})
	ms.team.seedMember(&model.TeamMember{
		TeamID: testTeamID,
		UserID: adminUserID,
	})

	ms.channel.seed(&model.Channel{
		ID:          testChannelID,
		TeamID:      testTeamID,
		CreatorID:   testUserID,
		Name:        "general",
		DisplayName: "General",
		Type:        "O",
		CreateAt:    1000,
		UpdateAt:    1000,
	})
	ms.channel.seedMember(&model.ChannelMember{
		ChannelID: testChannelID,
		UserID:    testUserID,
		CreateAt:  1000,
	})

	ms.post.seed(&model.Post{
		ID:        testRootPost,
		ChannelID: testChannelID,
		UserID:    testUserID,
		Content:   "Hello world",
		CreateAt:  2000,
		UpdateAt:  2000,
	})
	ms.post.seed(&model.Post{
		ID:        testReplyPost,
		ChannelID: testChannelID,
		UserID:    extraUserID,
		RootID:    testRootPost,
		Content:   "Hi there",
		CreateAt:  3000,
		UpdateAt:  3000,
	})

	ms.thread.seed(&model.Thread{
		PostID:      testRootPost,
		ChannelID:   testChannelID,
		ReplyCount:  1,
		LastReplyAt: 3000,
	})
	ms.thread.seedMembership(&model.ThreadMembership{
		PostID:    testRootPost,
		UserID:    testUserID,
		Following: true,
	})

	ms.tag.seed(&model.Tag{ID: testTagID, Name: "important"})
	ms.tag.seedPostTag(testRootPost, testTagID)

	cfg := config.Defaults()
	cfg.TrustedProxyHeader = "X-User-Id"
	hub := websocket.NewHub(nil)
	a := app.New(ms, hub, noopPubSub{}, cfg)

	cleanup := func() {
		hub.Stop()
	}

	return a, ms, cleanup
}

// authedRequest wraps the given request with the test user in context.
func authedRequest(r *http.Request, user *model.User) *http.Request {
	return ContextSetUser(r, user)
}

// testUser returns the pre-seeded test user.
func testUser() *model.User {
	return &model.User{
		ID:          testUserID,
		KratosID:    testKratosID,
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		CreateAt:    1000,
		UpdateAt:    1000,
	}
}

// decodeJSON reads and unmarshals a response body.
func decodeJSON(t *testing.T, body io.Reader, v any) {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal body (%s): %v", string(data), err)
	}
}
