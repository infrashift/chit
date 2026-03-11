package app

import (
	"reflect"
	"sort"
	"testing"

	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "basic mentions",
			content: "hello @alice and @bob",
			want:    []string{"alice", "bob"},
		},
		{
			name:    "dedup",
			content: "@alice @alice",
			want:    []string{"alice"},
		},
		{
			name:    "at all keyword",
			content: "hey @all please review",
			want:    []string{"all"},
		},
		{
			name:    "at channel keyword",
			content: "hey @channel please review",
			want:    []string{"channel"},
		},
		{
			name:    "no mentions",
			content: "hello world",
			want:    nil,
		},
		{
			name:    "email not matched",
			content: "email user@example.com please",
			want:    nil,
		},
		{
			name:    "case insensitive",
			content: "hi @Alice",
			want:    []string{"alice"},
		},
		{
			name:    "punctuation boundary before",
			content: "(@alice)",
			want:    []string{"alice"},
		},
		{
			name:    "start of string",
			content: "@alice hello",
			want:    []string{"alice"},
		},
		{
			name:    "mention with dots and dashes",
			content: "cc @john.doe-jr",
			want:    []string{"john.doe-jr"},
		},
		{
			name:    "multiple special keywords",
			content: "@all and @channel",
			want:    []string{"all", "channel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMentions(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMentions(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// testStore re-uses the mock types from the api package via a minimal re-implementation
// to keep app tests self-contained.

type mentionMockUserStore struct {
	users map[string]*model.User
}

func (s *mentionMockUserStore) Save(u *model.User) (*model.User, error)           { return u, nil }
func (s *mentionMockUserStore) Get(id string) (*model.User, error)                { return nil, errNotFound }
func (s *mentionMockUserStore) GetByKratosID(_ string) (*model.User, error)       { return nil, errNotFound }
func (s *mentionMockUserStore) GetByEmail(_ string) (*model.User, error)          { return nil, errNotFound }
func (s *mentionMockUserStore) Update(u *model.User) (*model.User, error)         { return u, nil }
func (s *mentionMockUserStore) Search(_ string, _, _ int) ([]*model.User, error)  { return nil, nil }
func (s *mentionMockUserStore) GetByIDs(_ []string) ([]*model.User, error)        { return nil, nil }
func (s *mentionMockUserStore) GetByUsername(username string) (*model.User, error) {
	u, ok := s.users[username]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

type mentionMockChannelStore struct {
	members map[string][]*model.ChannelMember
	mentionCounts map[string]int64 // key: channelID+":"+userID
}

func (s *mentionMockChannelStore) Save(_ *model.Channel) (*model.Channel, error)    { return nil, nil }
func (s *mentionMockChannelStore) Get(_ string) (*model.Channel, error)              { return nil, nil }
func (s *mentionMockChannelStore) Update(_ *model.Channel) (*model.Channel, error)   { return nil, nil }
func (s *mentionMockChannelStore) Delete(_ string, _ int64) error                    { return nil }
func (s *mentionMockChannelStore) GetChannelsForTeam(_ string, _, _ int) ([]*model.Channel, error) {
	return nil, nil
}
func (s *mentionMockChannelStore) GetChannelsForUser(_, _ string) ([]*model.Channel, error) {
	return nil, nil
}
func (s *mentionMockChannelStore) SaveMember(_ *model.ChannelMember) (*model.ChannelMember, error) {
	return nil, nil
}
func (s *mentionMockChannelStore) RemoveMember(_, _ string) error          { return nil }
func (s *mentionMockChannelStore) UpdateLastViewedAt(_, _ string, _ int64) error { return nil }
func (s *mentionMockChannelStore) GetByName(_, _ string) (*model.Channel, error) { return nil, errNotFound }
func (s *mentionMockChannelStore) GetDirectChannelByName(_ string) (*model.Channel, error) {
	return nil, errNotFound
}
func (s *mentionMockChannelStore) SaveDirectChannel(c *model.Channel, _ []string) (*model.Channel, error) {
	return c, nil
}
func (s *mentionMockChannelStore) GetDirectChannelsForUser(_ string) ([]*model.Channel, error) {
	return nil, nil
}
func (s *mentionMockChannelStore) IncrementMsgCount(_ string, _ int64) error { return nil }
func (s *mentionMockChannelStore) GetMembers(channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	all := s.members[channelID]
	start := page * perPage
	if start >= len(all) {
		return nil, nil
	}
	end := start + perPage
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], nil
}
func (s *mentionMockChannelStore) GetMember(channelID, userID string) (*model.ChannelMember, error) {
	for _, m := range s.members[channelID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, errNotFound
}
func (s *mentionMockChannelStore) IncrementMentionCount(channelID, userID string) error {
	if s.mentionCounts == nil {
		s.mentionCounts = make(map[string]int64)
	}
	s.mentionCounts[channelID+":"+userID]++
	return nil
}

type mentionMockThreadStore struct {
	memberships   map[string]*model.ThreadMembership
	mentionCounts map[string]int // key: postID+":"+userID
}

func (s *mentionMockThreadStore) SaveOrUpdate(_ *model.Thread) error       { return nil }
func (s *mentionMockThreadStore) Get(_ string) (*model.Thread, error)      { return nil, errNotFound }
func (s *mentionMockThreadStore) SaveMembership(_ *model.ThreadMembership) error { return nil }
func (s *mentionMockThreadStore) UpdateMembership(_ *model.ThreadMembership) error { return nil }
func (s *mentionMockThreadStore) GetThreadsForUser(_, _ string, _, _ int) (*model.UserThreadList, error) {
	return &model.UserThreadList{}, nil
}
func (s *mentionMockThreadStore) IncrementReplyCount(_ string, _ int64, _ string) error { return nil }
func (s *mentionMockThreadStore) MarkAsRead(_, _ string, _ int64) error                 { return nil }
func (s *mentionMockThreadStore) GetMembership(postID, userID string) (*model.ThreadMembership, error) {
	m, ok := s.memberships[postID+":"+userID]
	if !ok {
		return nil, errNotFound
	}
	return m, nil
}
func (s *mentionMockThreadStore) IncrementMentionCount(postID, userID string) error {
	if s.mentionCounts == nil {
		s.mentionCounts = make(map[string]int)
	}
	s.mentionCounts[postID+":"+userID]++
	return nil
}

// Minimal mock store assembly

type mentionMockTeamStore struct{}

func (mentionMockTeamStore) Save(_ *model.Team) (*model.Team, error)    { return nil, nil }
func (mentionMockTeamStore) Get(_ string) (*model.Team, error)          { return nil, nil }
func (mentionMockTeamStore) GetByName(_ string) (*model.Team, error)    { return nil, nil }
func (mentionMockTeamStore) Update(_ *model.Team) (*model.Team, error)  { return nil, nil }
func (mentionMockTeamStore) Delete(_ string, _ int64) error             { return nil }
func (mentionMockTeamStore) GetAll(_, _ int) ([]*model.Team, error)     { return nil, nil }
func (mentionMockTeamStore) GetTeamsForUser(_ string) ([]*model.Team, error) { return nil, nil }
func (mentionMockTeamStore) SaveMember(_ *model.TeamMember) (*model.TeamMember, error) {
	return nil, nil
}
func (mentionMockTeamStore) RemoveMember(_, _ string) error                        { return nil }
func (mentionMockTeamStore) GetMembers(_ string, _, _ int) ([]*model.TeamMember, error) { return nil, nil }
func (mentionMockTeamStore) GetMember(_, _ string) (*model.TeamMember, error) { return nil, errNotFound }

type mentionMockPostStore struct{}

func (mentionMockPostStore) Save(p *model.Post) (*model.Post, error) { return p, nil }
func (mentionMockPostStore) Get(_ string) (*model.Post, error)       { return nil, errNotFound }
func (mentionMockPostStore) Update(p *model.Post) (*model.Post, error) { return p, nil }
func (mentionMockPostStore) Delete(_ string, _ int64) error          { return nil }
func (mentionMockPostStore) GetPostsForChannel(_ string, _ model.GetPostsOptions) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (mentionMockPostStore) GetPostsForThread(_ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (mentionMockPostStore) GetPinnedPosts(_ string) (*model.PostList, error) {
	return &model.PostList{}, nil
}
func (mentionMockPostStore) SetPinned(_ string, _ bool) error { return nil }
func (mentionMockPostStore) SearchByContent(_, _ string, _, _ int) ([]*model.Post, error) {
	return nil, nil
}

type mentionMockTagStore struct{}

func (mentionMockTagStore) Save(_ *model.Tag) (*model.Tag, error)        { return nil, nil }
func (mentionMockTagStore) GetAll() ([]*model.Tag, error)                { return nil, nil }
func (mentionMockTagStore) AddTagToPost(_, _ string) error               { return nil }
func (mentionMockTagStore) RemoveTagFromPost(_, _ string) error          { return nil }
func (mentionMockTagStore) GetTagsForPost(_ string) ([]*model.Tag, error) { return nil, nil }
func (mentionMockTagStore) GetPostIDsByTags(_ []string, _, _ int) ([]string, error) { return nil, nil }
func (mentionMockTagStore) FilterPostIDsByTags(_ []string, _ []string) ([]string, error) { return nil, nil }

type mentionMockStore struct {
	user    *mentionMockUserStore
	team    mentionMockTeamStore
	channel *mentionMockChannelStore
	post    mentionMockPostStore
	thread  *mentionMockThreadStore
	tag     mentionMockTagStore
}

func (m *mentionMockStore) User() store.UserStore       { return m.user }
func (m *mentionMockStore) Team() store.TeamStore       { return m.team }
func (m *mentionMockStore) Channel() store.ChannelStore { return m.channel }
func (m *mentionMockStore) Post() store.PostStore       { return m.post }
func (m *mentionMockStore) Thread() store.ThreadStore   { return m.thread }
func (m *mentionMockStore) Tag() store.TagStore         { return m.tag }
func (m *mentionMockStore) Close()                      {}

var errNotFound = &model.AppError{Message: "not found"}

func TestProcessMentions(t *testing.T) {
	const (
		channelID = "ch-001"
		authorID  = "user-author"
		aliceID   = "user-alice"
		bobID     = "user-bob"
		charlieID = "user-charlie"
	)

	newTestApp := func(users map[string]*model.User, members []*model.ChannelMember) *App {
		hub := websocket.NewHub()
		t.Cleanup(hub.Stop)

		ms := &mentionMockStore{
			user: &mentionMockUserStore{users: users},
			channel: &mentionMockChannelStore{
				members: map[string][]*model.ChannelMember{
					channelID: members,
				},
			},
			thread: &mentionMockThreadStore{
				memberships: make(map[string]*model.ThreadMembership),
			},
		}
		cfg := config.Defaults()
		return New(ms, hub, nil, cfg)
	}

	t.Run("resolves usernames to user IDs", func(t *testing.T) {
		a := newTestApp(
			map[string]*model.User{
				"alice": {ID: aliceID, Username: "alice"},
				"bob":   {ID: bobID, Username: "bob"},
			},
			[]*model.ChannelMember{
				{ChannelID: channelID, UserID: authorID},
				{ChannelID: channelID, UserID: aliceID},
				{ChannelID: channelID, UserID: bobID},
			},
		)

		post := &model.Post{
			ChannelID: channelID,
			UserID:    authorID,
			Content:   "hey @alice and @bob",
		}

		ids, err := a.processMentions(post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sort.Strings(ids)
		want := []string{aliceID, bobID}
		sort.Strings(want)
		if !reflect.DeepEqual(ids, want) {
			t.Errorf("got %v, want %v", ids, want)
		}
	})

	t.Run("skips self-mention", func(t *testing.T) {
		a := newTestApp(
			map[string]*model.User{
				"author": {ID: authorID, Username: "author"},
				"alice":  {ID: aliceID, Username: "alice"},
			},
			[]*model.ChannelMember{
				{ChannelID: channelID, UserID: authorID},
				{ChannelID: channelID, UserID: aliceID},
			},
		)

		post := &model.Post{
			ChannelID: channelID,
			UserID:    authorID,
			Content:   "I am @author and @alice",
		}

		ids, err := a.processMentions(post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(ids) != 1 || ids[0] != aliceID {
			t.Errorf("got %v, want [%s]", ids, aliceID)
		}
	})

	t.Run("skips non-members", func(t *testing.T) {
		a := newTestApp(
			map[string]*model.User{
				"alice":   {ID: aliceID, Username: "alice"},
				"charlie": {ID: charlieID, Username: "charlie"},
			},
			[]*model.ChannelMember{
				{ChannelID: channelID, UserID: authorID},
				{ChannelID: channelID, UserID: aliceID},
				// charlie is NOT a member
			},
		)

		post := &model.Post{
			ChannelID: channelID,
			UserID:    authorID,
			Content:   "hey @alice and @charlie",
		}

		ids, err := a.processMentions(post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(ids) != 1 || ids[0] != aliceID {
			t.Errorf("got %v, want [%s]", ids, aliceID)
		}
	})

	t.Run("@all returns all channel members minus author", func(t *testing.T) {
		a := newTestApp(
			map[string]*model.User{},
			[]*model.ChannelMember{
				{ChannelID: channelID, UserID: authorID},
				{ChannelID: channelID, UserID: aliceID},
				{ChannelID: channelID, UserID: bobID},
			},
		)

		post := &model.Post{
			ChannelID: channelID,
			UserID:    authorID,
			Content:   "hey @all",
		}

		ids, err := a.processMentions(post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sort.Strings(ids)
		want := []string{aliceID, bobID}
		sort.Strings(want)
		if !reflect.DeepEqual(ids, want) {
			t.Errorf("got %v, want %v", ids, want)
		}
	})

	t.Run("unknown username silently skipped", func(t *testing.T) {
		a := newTestApp(
			map[string]*model.User{
				"alice": {ID: aliceID, Username: "alice"},
			},
			[]*model.ChannelMember{
				{ChannelID: channelID, UserID: authorID},
				{ChannelID: channelID, UserID: aliceID},
			},
		)

		post := &model.Post{
			ChannelID: channelID,
			UserID:    authorID,
			Content:   "hey @alice and @nonexistent",
		}

		ids, err := a.processMentions(post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(ids) != 1 || ids[0] != aliceID {
			t.Errorf("got %v, want [%s]", ids, aliceID)
		}
	})
}
