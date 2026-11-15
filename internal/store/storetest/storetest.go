// Package storetest is an in-memory store.Store for tests.
//
// It follows the SQL store's observable contract rather than being a stub:
// lookups that miss return a 404 *model.AppError, reads return copies (so a
// caller that mutates a result changes nothing until it calls Update),
// soft-deleted rows are invisible to reads, and the upserts keep what the SQL
// upserts keep. A test written against it fails for the same reasons a real
// deployment would, which hand-rolled per-test mocks answering (nil, nil) did
// not.
//
// Seed* methods load fixtures directly and are not part of the store
// interface.
package storetest

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
)

// Store is an in-memory store.Store. Construct it with New.
type Store struct {
	Users    *UserStore
	Teams    *TeamStore
	Channels *ChannelStore
	Posts    *PostStore
	Threads  *ThreadStore
	Tags     *TagStore
	Files    *FileStore
}

var _ store.Store = (*Store)(nil)

// New returns an empty store.
func New() *Store {
	s := &Store{
		Users:    &UserStore{byID: map[string]*model.User{}},
		Teams:    &TeamStore{byID: map[string]*model.Team{}, members: map[string][]*model.TeamMember{}},
		Channels: &ChannelStore{byID: map[string]*model.Channel{}, members: map[string][]*model.ChannelMember{}},
		Posts:    &PostStore{byID: map[string]*model.Post{}},
		Threads:  &ThreadStore{threads: map[string]*model.Thread{}, memberships: map[string]*model.ThreadMembership{}},
		Tags: &TagStore{tags: map[string]*model.Tag{}, postTags: map[string]map[string]string{},
			channelTags: map[string]map[string]string{}},
		Files: &FileStore{byID: map[string]*model.File{}, postOf: map[string]string{}},
	}
	s.Tags.postChannel = func(postID string) (string, bool) {
		s.Posts.mu.RLock()
		defer s.Posts.mu.RUnlock()
		p, ok := s.Posts.live(postID)
		if !ok {
			return "", false
		}
		return p.ChannelID, true
	}
	s.Posts.channels = s.Channels
	s.Posts.tags = s.Tags
	s.Posts.files = s.Files
	s.Channels.teamMembers = func(teamID string) ([]string, error) {
		members, err := s.Teams.GetMembers(context.Background(), teamID, 0, 0)
		ids := make([]string, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.UserID)
		}
		return ids, err
	}
	s.Channels.usernames = func(name string) (string, bool) {
		u, err := s.Users.GetByUsername(context.Background(), name)
		if err != nil {
			return "", false
		}
		return u.ID, true
	}
	return s
}

func (s *Store) User() store.UserStore       { return s.Users }
func (s *Store) Team() store.TeamStore       { return s.Teams }
func (s *Store) Channel() store.ChannelStore { return s.Channels }
func (s *Store) Post() store.PostStore       { return s.Posts }
func (s *Store) Thread() store.ThreadStore   { return s.Threads }
func (s *Store) Tag() store.TagStore         { return s.Tags }
func (s *Store) File() store.FileStore       { return s.Files }
func (s *Store) Close()                      {}

func notFound(where, id string) error { return model.NewNotFoundError("storetest."+where, id) }

func cp[T any](v *T) *T {
	c := *v
	return &c
}

// ─── Users ───────────────────────────────────────────────────────

type UserStore struct {
	mu   sync.RWMutex
	byID map[string]*model.User
}

// Seed stores u as given.
func (s *UserStore) Seed(u *model.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[u.ID] = cp(u)
}

func (s *UserStore) find(match func(*model.User) bool) *model.User {
	for _, u := range s.byID {
		if u.DeleteAt == 0 && match(u) {
			return u
		}
	}
	return nil
}

func (s *UserStore) Save(_ context.Context, u *model.User) (*model.User, error) {
	u.PreSave()
	if err := u.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taken(u) {
		return nil, model.NewConflictError("storetest.UserStore.Save", "username or email is already taken")
	}
	s.byID[u.ID] = cp(u)
	return u, nil
}

// taken reports whether another live user holds u's username, email,
// Kratos ID or OAuth client, the columns the SQL schema keeps unique.
func (s *UserStore) taken(u *model.User) bool {
	return s.find(func(o *model.User) bool {
		if o.ID == u.ID {
			return false
		}
		return o.Username == u.Username ||
			(u.Email != "" && o.Email == u.Email) ||
			(u.KratosID != "" && o.KratosID == u.KratosID) ||
			(u.OAuthClientID != "" && o.OAuthClientID == u.OAuthClientID)
	}) != nil
}

func (s *UserStore) getBy(where, value string, match func(*model.User) bool) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u := s.find(match); u != nil {
		return cp(u), nil
	}
	return nil, notFound(where, value)
}

func (s *UserStore) Get(_ context.Context, id string) (*model.User, error) {
	return s.getBy("UserStore.Get", id, func(u *model.User) bool { return u.ID == id })
}

func (s *UserStore) GetByKratosID(_ context.Context, id string) (*model.User, error) {
	return s.getBy("UserStore.GetByKratosID", id, func(u *model.User) bool { return id != "" && u.KratosID == id })
}

func (s *UserStore) KratosIDDeactivated(_ context.Context, id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.byID {
		if id != "" && u.KratosID == id && u.DeleteAt != 0 {
			return true, nil
		}
	}
	return false, nil
}

func (s *UserStore) GetByUsername(_ context.Context, name string) (*model.User, error) {
	return s.getBy("UserStore.GetByUsername", name, func(u *model.User) bool { return u.Username == name })
}

func (s *UserStore) GetByOAuthClientID(_ context.Context, id string) (*model.User, error) {
	return s.getBy("UserStore.GetByOAuthClientID", id, func(u *model.User) bool { return id != "" && u.OAuthClientID == id })
}

func (s *UserStore) Update(_ context.Context, u *model.User) (*model.User, error) {
	u.PreUpdate()
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.byID[u.ID]
	if !ok || old.DeleteAt != 0 {
		return nil, notFound("UserStore.Update", u.ID)
	}
	if s.taken(u) {
		return nil, model.NewConflictError("storetest.UserStore.Update", "username or email is already taken")
	}
	s.byID[u.ID] = cp(u)
	return u, nil
}

// Search matches username and display name, case-insensitively, as the SQL
// store does. Results are ordered by username.
func (s *UserStore) Search(_ context.Context, term string, page, perPage int) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	term = strings.ToLower(term)
	var out []*model.User
	for _, u := range s.byID {
		if u.DeleteAt == 0 && (strings.Contains(strings.ToLower(u.Username), term) ||
			strings.Contains(strings.ToLower(u.DisplayName), term)) {
			out = append(out, cp(u))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return paginate(out, page, perPage), nil
}

func (s *UserStore) GetByIDs(_ context.Context, ids []string) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.User
	for _, id := range ids {
		if u, ok := s.byID[id]; ok && u.DeleteAt == 0 {
			out = append(out, cp(u))
		}
	}
	return out, nil
}

// ─── Teams ───────────────────────────────────────────────────────

type TeamStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.Team
	members map[string][]*model.TeamMember
}

func (s *TeamStore) Seed(t *model.Team) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[t.ID] = cp(t)
}

func (s *TeamStore) SeedMember(m *model.TeamMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.TeamID] = append(s.members[m.TeamID], cp(m))
}

func (s *TeamStore) Save(_ context.Context, t *model.Team) (*model.Team, error) {
	t.PreSave()
	if err := t.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[t.ID] = cp(t)
	return t, nil
}

func (s *TeamStore) Get(_ context.Context, id string) (*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.byID[id]; ok && t.DeleteAt == 0 {
		return cp(t), nil
	}
	return nil, notFound("TeamStore.Get", id)
}

func (s *TeamStore) GetByName(_ context.Context, name string) (*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.byID {
		if t.DeleteAt == 0 && t.Name == name {
			return cp(t), nil
		}
	}
	return nil, notFound("TeamStore.GetByName", name)
}

func (s *TeamStore) Update(_ context.Context, t *model.Team) (*model.Team, error) {
	t.PreUpdate()
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.byID[t.ID]; !ok || old.DeleteAt != 0 {
		return nil, notFound("TeamStore.Update", t.ID)
	}
	s.byID[t.ID] = cp(t)
	return t, nil
}

func (s *TeamStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok || t.DeleteAt != 0 {
		return notFound("TeamStore.Delete", id)
	}
	t.DeleteAt = deleteAt
	return nil
}

func (s *TeamStore) isMember(teamID, userID string) bool {
	return slices.ContainsFunc(s.members[teamID], func(m *model.TeamMember) bool { return m.UserID == userID })
}

func (s *TeamStore) GetAll(_ context.Context, visibleTo string, page, perPage int) ([]*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Team
	for _, t := range s.byID {
		if t.DeleteAt != 0 {
			continue
		}
		if visibleTo != "" && t.Type != model.TeamOpen && !s.isMember(t.ID, visibleTo) {
			continue
		}
		out = append(out, cp(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return paginate(out, page, perPage), nil
}

func (s *TeamStore) GetTeamsForUser(_ context.Context, userID string) ([]*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Team
	for _, t := range s.byID {
		if t.DeleteAt == 0 && s.isMember(t.ID, userID) {
			out = append(out, cp(t))
		}
	}
	return out, nil
}

// SaveMember keeps an existing member's roles, as the SQL upsert does.
func (s *TeamStore) SaveMember(_ context.Context, m *model.TeamMember) (*model.TeamMember, error) {
	m.PreSave()
	if err := m.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.members[m.TeamID] {
		if existing.UserID == m.UserID {
			return cp(existing), nil
		}
	}
	s.members[m.TeamID] = append(s.members[m.TeamID], cp(m))
	return m, nil
}

func (s *TeamStore) RemoveMember(_ context.Context, teamID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[teamID] = slices.DeleteFunc(s.members[teamID], func(m *model.TeamMember) bool { return m.UserID == userID })
	return nil
}

func (s *TeamStore) GetMembers(_ context.Context, teamID string, page, perPage int) ([]*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.TeamMember, 0, len(s.members[teamID]))
	for _, m := range s.members[teamID] {
		out = append(out, cp(m))
	}
	return paginate(out, page, perPage), nil
}

func (s *TeamStore) GetMember(_ context.Context, teamID, userID string) (*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.members[teamID] {
		if m.UserID == userID {
			return cp(m), nil
		}
	}
	return nil, notFound("TeamStore.GetMember", teamID+"/"+userID)
}

// ─── Channels ────────────────────────────────────────────────────

type ChannelStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.Channel
	members map[string][]*model.ChannelMember
	// usernames resolves a live user by name; set by New.
	usernames func(name string) (id string, ok bool)
	// teamMembers lists a team's member IDs; set by New.
	teamMembers func(teamID string) ([]string, error)
}

func (s *ChannelStore) Seed(c *model.Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = cp(c)
}

func (s *ChannelStore) SeedMember(m *model.ChannelMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ChannelID] = append(s.members[m.ChannelID], cp(m))
}

func (s *ChannelStore) Save(_ context.Context, c *model.Channel) (*model.Channel, error) {
	c.PreSave()
	if err := c.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = cp(c)
	return c, nil
}

func (s *ChannelStore) live(id string) (*model.Channel, bool) {
	c, ok := s.byID[id]
	return c, ok && c.DeleteAt == 0
}

func (s *ChannelStore) Get(_ context.Context, id string) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.live(id); ok {
		return cp(c), nil
	}
	return nil, notFound("ChannelStore.Get", id)
}

func (s *ChannelStore) Update(_ context.Context, c *model.Channel) (*model.Channel, error) {
	c.PreUpdate()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.live(c.ID); !ok {
		return nil, notFound("ChannelStore.Update", c.ID)
	}
	s.byID[c.ID] = cp(c)
	return c, nil
}

func (s *ChannelStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.live(id)
	if !ok {
		return notFound("ChannelStore.Delete", id)
	}
	c.DeleteAt = deleteAt
	return nil
}

func (s *ChannelStore) isMember(channelID, userID string) bool {
	return slices.ContainsFunc(s.members[channelID], func(m *model.ChannelMember) bool { return m.UserID == userID })
}

func (s *ChannelStore) GetChannelsForTeam(_ context.Context, teamID string, page, perPage int) ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Channel
	for _, c := range s.byID {
		if c.DeleteAt == 0 && c.TeamID == teamID && c.Type == model.ChannelOpen {
			out = append(out, cp(c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return paginate(out, page, perPage), nil
}

func (s *ChannelStore) GetChannelsForUser(_ context.Context, userID, teamID string) ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Channel
	for _, c := range s.byID {
		if c.DeleteAt == 0 && c.TeamID == teamID && s.isMember(c.ID, userID) {
			out = append(out, cp(c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

// SaveMember is a no-op for an existing member, as the SQL store's
// ON CONFLICT DO NOTHING is.
func (s *ChannelStore) SaveMember(_ context.Context, m *model.ChannelMember) (*model.ChannelMember, error) {
	m.PreSave()
	if err := m.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isMember(m.ChannelID, m.UserID) {
		s.members[m.ChannelID] = append(s.members[m.ChannelID], cp(m))
	}
	return m, nil
}

func (s *ChannelStore) RemoveMember(_ context.Context, channelID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[channelID] = slices.DeleteFunc(s.members[channelID], func(m *model.ChannelMember) bool { return m.UserID == userID })
	return nil
}

func (s *ChannelStore) GetMembers(_ context.Context, channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.ChannelMember, 0, len(s.members[channelID]))
	for _, m := range s.members[channelID] {
		out = append(out, cp(m))
	}
	return paginate(out, page, perPage), nil
}

func (s *ChannelStore) GetMember(_ context.Context, channelID, userID string) (*model.ChannelMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.live(channelID); !ok {
		return nil, notFound("ChannelStore.GetMember", channelID+"/"+userID)
	}
	for _, m := range s.members[channelID] {
		if m.UserID == userID {
			return cp(m), nil
		}
	}
	return nil, notFound("ChannelStore.GetMember", channelID+"/"+userID)
}

func (s *ChannelStore) GetMembersForUser(_ context.Context, userID, teamID string) ([]*model.ChannelMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*model.ChannelMember{}
	for channelID, members := range s.members {
		if c, ok := s.live(channelID); !ok || c.TeamID != teamID {
			continue
		}
		for _, m := range members {
			if m.UserID == userID {
				out = append(out, cp(m))
			}
		}
	}
	return out, nil
}

func (s *ChannelStore) GetChannelIDsForUser(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for channelID := range s.members {
		if _, ok := s.live(channelID); ok && s.isMember(channelID, userID) {
			ids = append(ids, channelID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *ChannelStore) UpdateLastViewedAt(_ context.Context, channelID, userID string, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members[channelID] {
		if m.UserID == userID {
			m.LastViewedAt = at
			m.MentionCount = 0
			return nil
		}
	}
	return notFound("ChannelStore.UpdateLastViewedAt", channelID+"/"+userID)
}

func (s *ChannelStore) GetByName(_ context.Context, teamID, name string) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		if c.DeleteAt == 0 && c.TeamID == teamID && c.Name == name {
			return cp(c), nil
		}
	}
	return nil, notFound("ChannelStore.GetByName", name)
}

func (s *ChannelStore) SaveDirectChannel(_ context.Context, c *model.Channel, userIDs []string) (*model.Channel, error) {
	c.PreSave()
	if err := c.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = cp(c)
	for _, uid := range userIDs {
		s.members[c.ID] = append(s.members[c.ID], &model.ChannelMember{ChannelID: c.ID, UserID: uid})
	}
	return c, nil
}

func (s *ChannelStore) GetDirectChannelByName(_ context.Context, name string) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.byID {
		if c.DeleteAt == 0 && c.TeamID == "" && c.Name == name &&
			(c.Type == model.ChannelDirect || c.Type == model.ChannelGroup) {
			return cp(c), nil
		}
	}
	return nil, notFound("ChannelStore.GetDirectChannelByName", name)
}

func (s *ChannelStore) GetDirectChannelsForUser(_ context.Context, userID string) ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Channel
	for _, c := range s.byID {
		if c.DeleteAt == 0 && (c.Type == model.ChannelDirect || c.Type == model.ChannelGroup) && s.isMember(c.ID, userID) {
			out = append(out, cp(c))
		}
	}
	return out, nil
}

func (s *ChannelStore) AddTeamMembers(_ context.Context, channelID, teamID string) ([]string, error) {
	userIDs, err := s.teamMembers(teamID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var added []string
	for _, uid := range userIDs {
		if !s.isMember(channelID, uid) {
			s.members[channelID] = append(s.members[channelID], &model.ChannelMember{
				ChannelID: channelID, UserID: uid, Roles: "channel_user", CreateAt: model.GetMillis(),
			})
			added = append(added, uid)
		}
	}
	return added, nil
}

func (s *ChannelStore) DeleteForTeam(_ context.Context, teamID string, deleteAt int64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, c := range s.byID {
		if c.TeamID == teamID && c.DeleteAt == 0 {
			c.DeleteAt = deleteAt
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

func (s *ChannelStore) RemoveMemberFromTeam(_ context.Context, teamID, userID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, c := range s.byID {
		if c.TeamID == teamID && s.isMember(c.ID, userID) {
			s.members[c.ID] = slices.DeleteFunc(s.members[c.ID], func(m *model.ChannelMember) bool { return m.UserID == userID })
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

func (s *ChannelStore) IncrementMsgCount(_ context.Context, channelID string, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.live(channelID); ok {
		c.TotalMsgCount++
		c.LastPostAt = max(c.LastPostAt, at)
	}
	return nil
}

func (s *ChannelStore) IncrementMentionCounts(_ context.Context, channelID string, userIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members[channelID] {
		if slices.Contains(userIDs, m.UserID) {
			m.MentionCount++
		}
	}
	return nil
}

func (s *ChannelStore) GetMemberIDs(_ context.Context, channelID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.members[channelID]))
	for _, m := range s.members[channelID] {
		ids = append(ids, m.UserID)
	}
	return ids, nil
}

// GetMemberIDsByUsernames needs users, which this store does not hold; the
// Store wires users in through New.
func (s *ChannelStore) GetMemberIDsByUsernames(_ context.Context, channelID string, usernames []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for _, name := range usernames {
		if id, ok := s.usernames(name); ok && s.isMember(channelID, id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ─── Posts ───────────────────────────────────────────────────────

type PostStore struct {
	mu       sync.RWMutex
	byID     map[string]*model.Post
	channels *ChannelStore
	tags     *TagStore
	files    *FileStore
}

func (s *PostStore) Seed(p *model.Post) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[p.ID] = cp(p)
}

func (s *PostStore) Save(_ context.Context, p *model.Post) (*model.Post, error) {
	p.PreSave()
	if err := p.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.files.attach(p); err != nil {
		return nil, err
	}
	s.byID[p.ID] = cp(p)
	return p, nil
}

func (s *PostStore) live(id string) (*model.Post, bool) {
	p, ok := s.byID[id]
	return p, ok && p.DeleteAt == 0
}

func (s *PostStore) Get(_ context.Context, id string) (*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.live(id); ok {
		return cp(p), nil
	}
	return nil, notFound("PostStore.Get", id)
}

// Update writes content, props, hashtags and the edit timestamps, the
// columns the SQL UPDATE writes; everything else on the stored row is kept.
func (s *PostStore) Update(_ context.Context, p *model.Post) (*model.Post, error) {
	p.PreUpdate()
	p.EditAt = p.UpdateAt
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.live(p.ID)
	if !ok {
		return nil, notFound("PostStore.Update", p.ID)
	}
	old.Content, old.Props, old.Hashtags = p.Content, p.Props, p.Hashtags
	old.EditAt, old.UpdateAt = p.EditAt, p.UpdateAt
	return p, nil
}

func (s *PostStore) Delete(_ context.Context, id string, deleteAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.live(id)
	if !ok {
		return notFound("PostStore.Delete", id)
	}
	p.DeleteAt, p.UpdateAt = deleteAt, deleteAt
	s.files.deleteForPost(id, deleteAt)
	p.FileIDs = nil
	return nil
}

// list returns live posts matching keep, newest first.
func (s *PostStore) list(keep func(*model.Post) bool) []*model.Post {
	var out []*model.Post
	for _, p := range s.byID {
		if p.DeleteAt == 0 && keep(p) {
			out = append(out, cp(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreateAt > out[j].CreateAt })
	return out
}

func (s *PostStore) GetPostsForChannel(_ context.Context, channelID string, opts model.GetPostsOptions) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	posts := s.list(func(p *model.Post) bool { return p.ChannelID == channelID && p.CreateAt > opts.Since })
	// Pollers (Since > 0) read forward in time; history readers newest first.
	if opts.Since > 0 {
		slices.Reverse(posts)
	}
	return &model.PostList{Order: paginate(posts, opts.Page, opts.PerPage)}, nil
}

func (s *PostStore) GetPostsForThread(_ context.Context, rootID string) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	posts := s.list(func(p *model.Post) bool { return p.ID == rootID || p.RootID == rootID })
	slices.Reverse(posts)
	return &model.PostList{Order: posts}, nil
}

func (s *PostStore) GetPinnedPosts(_ context.Context, channelID string) (*model.PostList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &model.PostList{Order: s.list(func(p *model.Post) bool { return p.ChannelID == channelID && p.IsPinned })}, nil
}

func (s *PostStore) SetPinned(_ context.Context, id string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.live(id)
	if !ok {
		return notFound("PostStore.SetPinned", id)
	}
	p.IsPinned = pinned
	return nil
}

// GetPostsSince returns posts, deleted ones included, after the cursor in
// (update_at, id) order and updated no later than until.
func (s *PostStore) GetPostsSince(_ context.Context, after model.PostCursor, until int64, limit int) ([]*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Post
	for _, p := range s.byID {
		if p.UpdateAt <= until && (p.UpdateAt > after.UpdateAt || (p.UpdateAt == after.UpdateAt && p.ID > after.ID)) {
			out = append(out, cp(p))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdateAt != out[j].UpdateAt {
			return out[i].UpdateAt < out[j].UpdateAt
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *PostStore) GetByIDs(_ context.Context, ids []string) ([]*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Post
	for _, id := range ids {
		if p, ok := s.live(id); ok {
			out = append(out, cp(p))
		}
	}
	return out, nil
}

// Search applies every filter before paginating, as the SQL query does.
func (s *PostStore) Search(_ context.Context, q *model.PostSearch) ([]*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.tags.mu.RLock()
	defer s.tags.mu.RUnlock()
	terms := strings.ToLower(q.Terms)
	posts := s.list(func(p *model.Post) bool {
		return slices.Contains(q.ChannelIDs, p.ChannelID) &&
			strings.Contains(strings.ToLower(p.Content), terms) &&
			(q.AuthorID == "" || p.UserID == q.AuthorID) &&
			s.tags.hasAll(p.ID, q.TagIDs)
	})
	return paginate(posts, q.Page, q.PerPage), nil
}

// ─── Files ───────────────────────────────────────────────────────

type FileStore struct {
	mu     sync.RWMutex
	byID   map[string]*model.File
	postOf map[string]string // file ID → post ID
}

// Seed stores f as given.
func (s *FileStore) Seed(f *model.File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[f.ID] = cp(f)
}

func (s *FileStore) out(f *model.File) *model.File {
	c := cp(f)
	c.PostID = s.postOf[f.ID]
	return c
}

func (s *FileStore) live(id string) (*model.File, bool) {
	f, ok := s.byID[id]
	return f, ok && f.DeleteAt == 0
}

// attach links p.FileIDs to p under the SQL INSERT's conditions.
func (s *FileStore) attach(p *model.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, id := range p.FileIDs {
		f, ok := s.live(id)
		if !ok || seen[id] || f.ChannelID != p.ChannelID || f.UploaderID != p.UserID || s.postOf[id] != "" {
			return model.NewConflictError("storetest.PostStore.Save",
				"a file is gone, attached to another post, or not yours to attach in this channel")
		}
		seen[id] = true
	}
	for _, id := range p.FileIDs {
		s.postOf[id] = p.ID
	}
	return nil
}

func (s *FileStore) deleteForPost(postID string, deleteAt int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for fileID, pid := range s.postOf {
		if pid != postID {
			continue
		}
		delete(s.postOf, fileID)
		if f, ok := s.live(fileID); ok {
			f.DeleteAt = deleteAt
		}
	}
}

func (s *FileStore) Save(_ context.Context, f *model.File) (*model.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[f.ID] = cp(f)
	return f, nil
}

func (s *FileStore) Get(_ context.Context, id string) (*model.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if f, ok := s.live(id); ok {
		return s.out(f), nil
	}
	return nil, notFound("FileStore.Get", id)
}

func (s *FileStore) GetByIDs(_ context.Context, ids []string) ([]*model.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.File
	for _, id := range ids {
		if f, ok := s.live(id); ok {
			out = append(out, s.out(f))
		}
	}
	return out, nil
}

func (s *FileStore) GetBySHA256(_ context.Context, channelID, sum string) (*model.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best *model.File
	for _, f := range s.byID {
		if f.DeleteAt == 0 && f.ChannelID == channelID && f.SHA256 == sum &&
			(best == nil || f.CreateAt < best.CreateAt || (f.CreateAt == best.CreateAt && f.ID < best.ID)) {
			best = f
		}
	}
	if best == nil {
		return nil, notFound("FileStore.GetBySHA256", sum)
	}
	return s.out(best), nil
}

func (s *FileStore) UsageBytes(_ context.Context, uploaderID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for _, f := range s.byID {
		if f.DeleteAt == 0 && f.UploaderID == uploaderID {
			n += f.SizeBytes
		}
	}
	return n, nil
}

func (s *FileStore) Search(_ context.Context, q *model.FileSearch) ([]*model.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	terms := strings.ToLower(q.Terms)
	var out []*model.File
	for _, f := range s.byID {
		if f.DeleteAt == 0 && slices.Contains(q.ChannelIDs, f.ChannelID) &&
			(strings.Contains(strings.ToLower(f.Name), terms) || strings.Contains(strings.ToLower(f.AltText), terms)) &&
			(f.UploaderID == q.ReaderID || s.postOf[f.ID] != "") {
			out = append(out, s.out(f))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreateAt != out[j].CreateAt {
			return out[i].CreateAt > out[j].CreateAt
		}
		return out[i].ID > out[j].ID
	})
	return paginate(out, q.Page, q.PerPage), nil
}

func (s *FileStore) ExpireUnattached(_ context.Context, createdBefore, deleteAt int64, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cand []*model.File
	for _, f := range s.byID {
		if f.DeleteAt == 0 && f.CreateAt < createdBefore && s.postOf[f.ID] == "" {
			cand = append(cand, f)
		}
	}
	sort.Slice(cand, func(i, j int) bool { return cand[i].CreateAt < cand[j].CreateAt })
	if len(cand) > limit {
		cand = cand[:limit]
	}
	for _, f := range cand {
		f.DeleteAt = deleteAt
	}
	return len(cand), nil
}

func (s *FileStore) Deleted(_ context.Context, deletedBefore int64, limit int) ([]*model.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.File
	for _, f := range s.byID {
		if f.DeleteAt > 0 && f.DeleteAt < deletedBefore {
			out = append(out, s.out(f))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeleteAt < out[j].DeleteAt })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *FileStore) ObjectInUse(_ context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.byID {
		if f.DeleteAt == 0 && f.S3Key == key {
			return true, nil
		}
	}
	return false, nil
}

func (s *FileStore) Purge(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.byID[id]; ok && f.DeleteAt > 0 {
		delete(s.byID, id)
		delete(s.postOf, id)
	}
	return nil
}

// ─── Threads ─────────────────────────────────────────────────────

type ThreadStore struct {
	mu          sync.RWMutex
	threads     map[string]*model.Thread
	memberships map[string]*model.ThreadMembership
}

func membershipKey(postID, userID string) string { return postID + ":" + userID }

func (s *ThreadStore) Seed(t *model.Thread) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threads[t.PostID] = cp(t)
}

func (s *ThreadStore) SeedMembership(m *model.ThreadMembership) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberships[membershipKey(m.PostID, m.UserID)] = cp(m)
}

// SaveOrUpdate only ensures the thread row exists, as the SQL store's
// ON CONFLICT DO NOTHING does.
func (s *ThreadStore) SaveOrUpdate(_ context.Context, t *model.Thread) error {
	if err := t.IsValid(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.threads[t.PostID]; !ok {
		s.threads[t.PostID] = cp(t)
	}
	return nil
}

func (s *ThreadStore) Get(_ context.Context, postID string) (*model.Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.threads[postID]; ok {
		return cp(t), nil
	}
	return nil, notFound("ThreadStore.Get", postID)
}

func (s *ThreadStore) SaveMembership(_ context.Context, m *model.ThreadMembership) error {
	if err := m.IsValid(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := membershipKey(m.PostID, m.UserID)
	if old, ok := s.memberships[key]; ok {
		old.Following = m.Following
		old.LastViewedAt = max(old.LastViewedAt, m.LastViewedAt)
		return nil
	}
	s.memberships[key] = cp(m)
	return nil
}

func (s *ThreadStore) GetMembership(_ context.Context, postID, userID string) (*model.ThreadMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.memberships[membershipKey(postID, userID)]; ok {
		return cp(m), nil
	}
	return nil, notFound("ThreadStore.GetMembership", postID+"/"+userID)
}

func (s *ThreadStore) UpdateMembership(_ context.Context, m *model.ThreadMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := membershipKey(m.PostID, m.UserID)
	if _, ok := s.memberships[key]; !ok {
		return notFound("ThreadStore.UpdateMembership", key)
	}
	s.memberships[key] = cp(m)
	return nil
}

// GetThreadsForUser lists the threads the user follows. It does not join
// channels or posts, so it applies no team or access filter; that query is
// covered by the sqlstore integration tests.
func (s *ThreadStore) GetThreadsForUser(_ context.Context, userID, _ string, page, perPage int) (*model.UserThreadList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.ThreadResponse
	for _, m := range s.memberships {
		t, ok := s.threads[m.PostID]
		if m.UserID != userID || !m.Following || !ok {
			continue
		}
		out = append(out, &model.ThreadResponse{Thread: cp(t), LastViewedAt: m.LastViewedAt, UnreadMentions: m.UnreadMentionCount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Thread.LastReplyAt > out[j].Thread.LastReplyAt })
	return &model.UserThreadList{Threads: paginate(out, page, perPage), Total: int64(len(out))}, nil
}

// GetDirectThreadsForUser is GetThreadsForUser: this store cannot tell a
// team's channels from direct ones (the SQL scoping is covered by the
// sqlstore integration tests).
func (s *ThreadStore) GetDirectThreadsForUser(ctx context.Context, userID string, page, perPage int) (*model.UserThreadList, error) {
	return s.GetThreadsForUser(ctx, userID, "", page, perPage)
}

func (s *ThreadStore) IncrementReplyCount(_ context.Context, postID string, at int64, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.threads[postID]; ok {
		t.ReplyCount++
		t.LastReplyAt = max(t.LastReplyAt, at)
		if !slices.Contains(t.Participants, userID) {
			t.Participants = append(t.Participants, userID)
		}
	}
	return nil
}

// DecrementReplyCount lowers the count. Unlike the SQL store it cannot see
// posts, so last_reply_at is left as it is.
func (s *ThreadStore) DecrementReplyCount(_ context.Context, postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.threads[postID]; ok && t.ReplyCount > 0 {
		t.ReplyCount--
	}
	return nil
}

func (s *ThreadStore) MarkAsRead(_ context.Context, postID, userID string, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.memberships[membershipKey(postID, userID)]
	if !ok {
		return notFound("ThreadStore.MarkAsRead", postID+"/"+userID)
	}
	m.LastViewedAt, m.UnreadMentionCount = at, 0
	return nil
}

func (s *ThreadStore) IncrementMentionCounts(_ context.Context, postID string, userIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range userIDs {
		if m, ok := s.memberships[membershipKey(postID, id)]; ok && m.Following {
			m.UnreadMentionCount++
		}
	}
	return nil
}

// ─── Tags ────────────────────────────────────────────────────────

type TagStore struct {
	mu          sync.RWMutex
	tags        map[string]*model.Tag
	postTags    map[string]map[string]string // post -> tag -> applied_by
	channelTags map[string]map[string]string // channel -> tag -> applied_by
	// postChannel resolves a live post to its channel, for
	// FilterChannelIDsByTags. Set by New; called without the tag lock.
	postChannel func(postID string) (string, bool)
}

func (s *TagStore) Seed(t *model.Tag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := cp(t)
	if c.Scope == "" {
		c.Scope = model.TagScopePrivate
	}
	if c.NameKey == "" {
		c.NameKey = model.TagNameKey(c.Name)
	}
	s.tags[t.ID] = c
}

// SeedPostTag tags a post.
func (s *TagStore) SeedPostTag(postID, tagID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tagLocked(postID, tagID, "")
}

// SeedChannelTag tags a channel.
func (s *TagStore) SeedChannelTag(channelID, tagID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.channelTags[channelID] == nil {
		s.channelTags[channelID] = map[string]string{}
	}
	s.channelTags[channelID][tagID] = ""
}

// AppliedBy returns who applied tagID to the post or channel itemID, and
// whether it is applied there at all.
func (s *TagStore) AppliedBy(itemID, tagID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if by, ok := s.postTags[itemID][tagID]; ok {
		return by, true
	}
	by, ok := s.channelTags[itemID][tagID]
	return by, ok
}

func (s *TagStore) tagLocked(postID, tagID, appliedBy string) {
	if s.postTags[postID] == nil {
		s.postTags[postID] = map[string]string{}
	}
	if _, ok := s.postTags[postID][tagID]; !ok {
		s.postTags[postID][tagID] = appliedBy
	}
}

func (s *TagStore) keyTakenLocked(t *model.Tag) bool {
	for _, e := range s.tags {
		if e.ID == t.ID || e.Scope != t.Scope || e.NameKey != t.NameKey {
			continue
		}
		if (t.Scope == model.TagScopePrivate && e.OwnerID == t.OwnerID) ||
			(t.Scope == model.TagScopeChannel && e.ChannelID == t.ChannelID) {
			return true
		}
	}
	return false
}

func (s *TagStore) Save(_ context.Context, t *model.Tag) (*model.Tag, error) {
	t.PreSave()
	if err := t.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keyTakenLocked(t) {
		return nil, model.NewConflictError("TagStore.Save", "a tag with this name already exists")
	}
	s.tags[t.ID] = cp(t)
	return t, nil
}

func (s *TagStore) Get(_ context.Context, id string) (*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tags[id]
	if !ok {
		return nil, model.NewNotFoundError("TagStore.Get", id)
	}
	return cp(t), nil
}

func (s *TagStore) GetByIDs(_ context.Context, ids []string) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.Tag
	for _, id := range ids {
		if t, ok := s.tags[id]; ok {
			out = append(out, cp(t))
		}
	}
	return out, nil
}

func (s *TagStore) GetByKey(_ context.Context, scope, ownerID, channelID, nameKey string) (*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.tags {
		if e.Scope == scope && e.NameKey == nameKey &&
			((scope == model.TagScopePrivate && e.OwnerID == ownerID) ||
				(scope == model.TagScopeChannel && e.ChannelID == channelID)) {
			return cp(e), nil
		}
	}
	return nil, model.NewNotFoundError("TagStore.GetByKey", nameKey)
}

func (s *TagStore) Update(_ context.Context, t *model.Tag) (*model.Tag, error) {
	t.Name = model.NormalizeTagName(t.Name)
	t.NameKey = model.TagNameKey(t.Name)
	t.UpdateAt = model.GetMillis()
	if err := t.IsValid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tags[t.ID]; !ok {
		return nil, model.NewNotFoundError("TagStore.Update", t.ID)
	}
	if s.keyTakenLocked(t) {
		return nil, model.NewConflictError("TagStore.Update", "a tag with this name already exists")
	}
	s.tags[t.ID] = cp(t)
	return t, nil
}

func (s *TagStore) deleteLocked(id string) {
	delete(s.tags, id)
	for _, m := range s.postTags {
		delete(m, id)
	}
	for _, m := range s.channelTags {
		delete(m, id)
	}
}

func (s *TagStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteLocked(id)
	return nil
}

func (s *TagStore) DeleteByOwner(_ context.Context, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tags {
		if t.OwnerID == ownerID {
			s.deleteLocked(id)
		}
	}
	for _, apps := range []map[string]map[string]string{s.postTags, s.channelTags} {
		for _, m := range apps {
			for tagID, by := range m {
				if by == ownerID {
					m[tagID] = ""
				}
			}
		}
	}
	return nil
}

func (s *TagStore) CountByOwner(_ context.Context, ownerID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.tags {
		if t.OwnerID == ownerID {
			n++
		}
	}
	return n, nil
}

func sortTags(out []*model.Tag) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].NameKey != out[j].NameKey {
			return out[i].NameKey < out[j].NameKey
		}
		return out[i].ID < out[j].ID
	})
}

func (s *TagStore) GetVisible(_ context.Context, userID string, channelIDs []string) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	in := map[string]bool{}
	for _, id := range channelIDs {
		in[id] = true
	}
	var out []*model.Tag
	for _, t := range s.tags {
		if (t.Scope == model.TagScopePrivate && t.OwnerID == userID) ||
			(t.Scope == model.TagScopeChannel && in[t.ChannelID]) {
			out = append(out, cp(t))
		}
	}
	sortTags(out)
	return out, nil
}

func (s *TagStore) AddTagToPost(_ context.Context, postID, tagID, appliedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tagLocked(postID, tagID, appliedBy)
	return nil
}

func (s *TagStore) RemoveTagFromPost(_ context.Context, postID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.postTags[postID], tagID)
	return nil
}

func (s *TagStore) GetTagsForPost(_ context.Context, postID string) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tagsForLocked(s.postTags[postID]), nil
}

func (s *TagStore) tagsForLocked(applied map[string]string) []*model.Tag {
	var out []*model.Tag
	for tagID := range applied {
		if t, ok := s.tags[tagID]; ok {
			out = append(out, cp(t))
		}
	}
	sortTags(out)
	return out
}

func (s *TagStore) GetTagsForPosts(_ context.Context, postIDs []string) (map[string][]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string][]*model.Tag{}
	for _, id := range postIDs {
		if tags := s.tagsForLocked(s.postTags[id]); len(tags) > 0 {
			out[id] = tags
		}
	}
	return out, nil
}

func (s *TagStore) AddTagToChannel(_ context.Context, channelID, tagID, appliedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.channelTags[channelID] == nil {
		s.channelTags[channelID] = map[string]string{}
	}
	if _, ok := s.channelTags[channelID][tagID]; !ok {
		s.channelTags[channelID][tagID] = appliedBy
	}
	return nil
}

func (s *TagStore) RemoveTagFromChannel(_ context.Context, channelID, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.channelTags[channelID], tagID)
	return nil
}

func (s *TagStore) GetTagsForChannel(_ context.Context, channelID string) ([]*model.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tagsForLocked(s.channelTags[channelID]), nil
}

// hasAll reports whether postID carries every tag in tagIDs.
func (s *TagStore) hasAll(postID string, tagIDs []string) bool {
	for _, tagID := range tagIDs {
		if _, ok := s.postTags[postID][tagID]; !ok {
			return false
		}
	}
	return true
}

func (s *TagStore) FilterPostIDsByTags(_ context.Context, postIDs, tagIDs []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for _, postID := range postIDs {
		if s.hasAll(postID, tagIDs) {
			ids = append(ids, postID)
		}
	}
	return ids, nil
}

func (s *TagStore) FilterChannelIDsByTags(_ context.Context, channelIDs, tagIDs []string) ([]string, error) {
	want := map[string]bool{}
	for _, id := range tagIDs {
		want[id] = true
	}
	// Copy under the tag lock, then resolve posts without it: PostStore.Search
	// takes the post lock before the tag lock.
	s.mu.RLock()
	has := map[string]map[string]bool{}
	note := func(channelID, tagID string) {
		if has[channelID] == nil {
			has[channelID] = map[string]bool{}
		}
		has[channelID][tagID] = true
	}
	for channelID, m := range s.channelTags {
		for tagID := range m {
			if want[tagID] {
				note(channelID, tagID)
			}
		}
	}
	postHits := map[string][]string{}
	for postID, m := range s.postTags {
		for tagID := range m {
			if want[tagID] {
				postHits[postID] = append(postHits[postID], tagID)
			}
		}
	}
	s.mu.RUnlock()
	for postID, tags := range postHits {
		if channelID, live := s.postChannel(postID); live {
			for _, tagID := range tags {
				note(channelID, tagID)
			}
		}
	}
	var ids []string
	for _, channelID := range channelIDs {
		if len(has[channelID]) == len(want) {
			ids = append(ids, channelID)
		}
	}
	return ids, nil
}

// paginate returns page (zero-based) of items. A perPage of 0 or less returns
// everything, which lets fixtures call store methods without caring.
func paginate[T any](items []T, page, perPage int) []T {
	if perPage <= 0 {
		return items
	}
	start := page * perPage
	if start >= len(items) {
		return nil
	}
	return items[start:min(start+perPage, len(items))]
}
