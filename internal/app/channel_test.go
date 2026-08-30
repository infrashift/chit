package app

import (
	"context"
	"testing"

	"github.com/infrashift/chit/internal/config"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
	"github.com/infrashift/chit/internal/websocket"
)

// --- channel-specific mock stores ---

type chMockChannelStore struct {
	saved   []*model.Channel
	members []*model.ChannelMember
}

func (s *chMockChannelStore) Save(_ context.Context, c *model.Channel) (*model.Channel, error) {
	c.ID = "ch-new"
	s.saved = append(s.saved, c)
	return c, nil
}
func (s *chMockChannelStore) Get(_ context.Context, _ string) (*model.Channel, error) {
	return nil, errNotFound
}
func (s *chMockChannelStore) Update(_ context.Context, c *model.Channel) (*model.Channel, error) {
	return c, nil
}
func (s *chMockChannelStore) Delete(_ context.Context, _ string, _ int64) error { return nil }
func (s *chMockChannelStore) GetChannelsForTeam(_ context.Context, _ string, _, _ int) ([]*model.Channel, error) {
	return nil, nil
}
func (s *chMockChannelStore) GetChannelsForUser(_ context.Context, _, _ string) ([]*model.Channel, error) {
	return nil, nil
}
func (s *chMockChannelStore) SaveMember(_ context.Context, m *model.ChannelMember) (*model.ChannelMember, error) {
	s.members = append(s.members, m)
	return m, nil
}
func (s *chMockChannelStore) RemoveMember(_ context.Context, _, _ string) error { return nil }
func (s *chMockChannelStore) UpdateLastViewedAt(_ context.Context, _, _ string, _ int64) error {
	return nil
}
func (s *chMockChannelStore) GetByName(_ context.Context, _, _ string) (*model.Channel, error) {
	return nil, errNotFound
}
func (s *chMockChannelStore) GetDirectChannelByName(_ context.Context, _ string) (*model.Channel, error) {
	return nil, errNotFound
}
func (s *chMockChannelStore) SaveDirectChannel(_ context.Context, c *model.Channel, _ []string) (*model.Channel, error) {
	return c, nil
}
func (s *chMockChannelStore) GetDirectChannelsForUser(_ context.Context, _ string) ([]*model.Channel, error) {
	return nil, nil
}
func (s *chMockChannelStore) IncrementMsgCount(_ context.Context, _ string, _ int64) error {
	return nil
}
func (s *chMockChannelStore) GetMembers(_ context.Context, _ string, _, _ int) ([]*model.ChannelMember, error) {
	return nil, nil
}
func (s *chMockChannelStore) GetMember(_ context.Context, _, _ string) (*model.ChannelMember, error) {
	return nil, errNotFound
}
func (s *chMockChannelStore) IncrementMentionCount(_ context.Context, _, _ string) error { return nil }
func (s *chMockChannelStore) GetChannelIDsForUser(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

type chMockTeamStore struct {
	members []*model.TeamMember
}

func (s *chMockTeamStore) Save(_ context.Context, _ *model.Team) (*model.Team, error) {
	return nil, nil
}
func (s *chMockTeamStore) Get(_ context.Context, _ string) (*model.Team, error) { return nil, nil }
func (s *chMockTeamStore) GetByName(_ context.Context, _ string) (*model.Team, error) {
	return nil, nil
}
func (s *chMockTeamStore) Update(_ context.Context, _ *model.Team) (*model.Team, error) {
	return nil, nil
}
func (s *chMockTeamStore) Delete(_ context.Context, _ string, _ int64) error         { return nil }
func (s *chMockTeamStore) GetAll(_ context.Context, _, _ int) ([]*model.Team, error) { return nil, nil }
func (s *chMockTeamStore) GetTeamsForUser(_ context.Context, _ string) ([]*model.Team, error) {
	return nil, nil
}
func (s *chMockTeamStore) SaveMember(_ context.Context, _ *model.TeamMember) (*model.TeamMember, error) {
	return nil, nil
}
func (s *chMockTeamStore) RemoveMember(_ context.Context, _, _ string) error { return nil }
func (s *chMockTeamStore) GetMembers(_ context.Context, _ string, _, _ int) ([]*model.TeamMember, error) {
	return s.members, nil
}
func (s *chMockTeamStore) GetMember(_ context.Context, teamID, userID string) (*model.TeamMember, error) {
	for _, m := range s.members {
		if m.TeamID == teamID && m.UserID == userID {
			return m, nil
		}
	}
	return nil, errNotFound
}

type chMockStore struct {
	channel *chMockChannelStore
	team    *chMockTeamStore
}

func (s *chMockStore) User() store.UserStore       { return &mentionMockUserStore{} }
func (s *chMockStore) Team() store.TeamStore       { return s.team }
func (s *chMockStore) Channel() store.ChannelStore { return s.channel }
func (s *chMockStore) Post() store.PostStore       { return mentionMockPostStore{} }
func (s *chMockStore) Thread() store.ThreadStore   { return &mentionMockThreadStore{} }
func (s *chMockStore) Tag() store.TagStore         { return mentionMockTagStore{} }
func (s *chMockStore) Close()                      {}

func TestCreateChannel_OpenAutoAddsTeamMembers(t *testing.T) {
	cs := &chMockChannelStore{}
	ts := &chMockTeamStore{
		members: []*model.TeamMember{
			{TeamID: "team1", UserID: "creator"},
			{TeamID: "team1", UserID: "alice"},
			{TeamID: "team1", UserID: "bob"},
		},
	}
	ms := &chMockStore{channel: cs, team: ts}

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	cfg := config.Defaults()
	a := New(ms, hub, nil, cfg)

	ch := &model.Channel{
		TeamID:      "team1",
		Name:        "open-chan",
		DisplayName: "Open Chan",
		Type:        model.ChannelOpen,
		CreatorID:   "creator",
	}

	created, err := a.CreateChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "ch-new" {
		t.Errorf("channel ID = %q, want ch-new", created.ID)
	}

	// Should have 3 members: creator + alice + bob
	if len(cs.members) != 3 {
		t.Fatalf("expected 3 channel members, got %d", len(cs.members))
	}

	memberIDs := make(map[string]bool)
	for _, m := range cs.members {
		memberIDs[m.UserID] = true
	}
	for _, uid := range []string{"creator", "alice", "bob"} {
		if !memberIDs[uid] {
			t.Errorf("expected %s to be a member", uid)
		}
	}
}

func TestCreateChannel_PrivateOnlyAddsCreator(t *testing.T) {
	cs := &chMockChannelStore{}
	ts := &chMockTeamStore{
		members: []*model.TeamMember{
			{TeamID: "team1", UserID: "creator"},
			{TeamID: "team1", UserID: "alice"},
		},
	}
	ms := &chMockStore{channel: cs, team: ts}

	hub := websocket.NewHub(nil)
	t.Cleanup(hub.Stop)

	cfg := config.Defaults()
	a := New(ms, hub, nil, cfg)

	ch := &model.Channel{
		TeamID:      "team1",
		Name:        "private-chan",
		DisplayName: "Private Chan",
		Type:        model.ChannelPrivate,
		CreatorID:   "creator",
	}

	_, err := a.CreateChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have the creator
	if len(cs.members) != 1 {
		t.Fatalf("expected 1 channel member, got %d", len(cs.members))
	}
	if cs.members[0].UserID != "creator" {
		t.Errorf("expected creator, got %s", cs.members[0].UserID)
	}
}
