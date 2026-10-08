//go:build integration

package sqlstore

import (
	"errors"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestChannelStoreIntegration_MembershipCRUD(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	other, err := ss.User().Save(t.Context(), newTestUser("member2"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}

	// Creator fixture does not add membership; add both users.
	for _, uid := range []string{user.ID, other.ID} {
		if _, err = ss.Channel().SaveMember(t.Context(), &model.ChannelMember{
			ChannelID: channel.ID,
			UserID:    uid,
		}); err != nil {
			t.Fatalf("SaveMember(%s): %v", uid, err)
		}
	}

	// GetMember round-trip
	m, err := ss.Channel().GetMember(t.Context(), channel.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if m.UserID != user.ID || m.ChannelID != channel.ID {
		t.Errorf("GetMember: got %+v", m)
	}

	// GetMembers pagination
	members, err := ss.Channel().GetMembers(t.Context(), channel.ID, 0, 10)
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("GetMembers: got %d, want 2", len(members))
	}

	// GetChannelIDsForUser
	ids, err := ss.Channel().GetChannelIDsForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetChannelIDsForUser: %v", err)
	}
	if len(ids) != 1 || ids[0] != channel.ID {
		t.Errorf("GetChannelIDsForUser: got %v, want [%s]", ids, channel.ID)
	}

	// RemoveMember
	if err := ss.Channel().RemoveMember(t.Context(), channel.ID, other.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, err := ss.Channel().GetMember(t.Context(), channel.ID, other.ID); err == nil {
		t.Error("GetMember after RemoveMember: expected not-found error, got nil")
	}
}

func TestChannelStoreIntegration_GetChannelsForUser(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	if _, err := ss.Channel().SaveMember(t.Context(), &model.ChannelMember{
		ChannelID: channel.ID,
		UserID:    user.ID,
	}); err != nil {
		t.Fatalf("SaveMember: %v", err)
	}

	// Second channel in the team the user is NOT a member of.
	if _, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID:      channel.TeamID,
		Name:        "other-channel",
		DisplayName: "Other",
		Type:        model.ChannelOpen,
		CreatorID:   user.ID,
	}); err != nil {
		t.Fatalf("save other channel: %v", err)
	}

	channels, err := ss.Channel().GetChannelsForUser(t.Context(), user.ID, channel.TeamID)
	if err != nil {
		t.Fatalf("GetChannelsForUser: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != channel.ID {
		t.Errorf("GetChannelsForUser: got %d channels, want only %s", len(channels), channel.ID)
	}
}

func TestChannelStoreIntegration_IncrementMsgCount(t *testing.T) {
	ss := testStore(t)
	_, channel := newTestChannelFixture(t, ss)

	now := model.GetMillis()
	for i := 0; i < 3; i++ {
		if err := ss.Channel().IncrementMsgCount(t.Context(), channel.ID, now+int64(i)); err != nil {
			t.Fatalf("IncrementMsgCount: %v", err)
		}
	}

	got, err := ss.Channel().Get(t.Context(), channel.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TotalMsgCount != 3 {
		t.Errorf("TotalMsgCount: got %d, want 3", got.TotalMsgCount)
	}
	if got.LastPostAt != now+2 {
		t.Errorf("LastPostAt: got %d, want %d", got.LastPostAt, now+2)
	}
}

func TestChannelStoreIntegration_DirectChannel(t *testing.T) {
	ss := testStore(t)
	user, _ := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("dmpeer"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}

	dm, err := ss.Channel().SaveDirectChannel(t.Context(), &model.Channel{
		Name:        user.ID + "__" + other.ID,
		DisplayName: "DM",
		Type:        model.ChannelDirect,
		CreatorID:   user.ID,
	}, []string{user.ID, other.ID})
	if err != nil {
		t.Fatalf("SaveDirectChannel: %v", err)
	}

	// Both users are members atomically.
	for _, uid := range []string{user.ID, other.ID} {
		if _, err = ss.Channel().GetMember(t.Context(), dm.ID, uid); err != nil {
			t.Errorf("GetMember(%s): %v", uid, err)
		}
	}

	found, err := ss.Channel().GetDirectChannelByName(t.Context(), dm.Name)
	if err != nil {
		t.Fatalf("GetDirectChannelByName: %v", err)
	}
	if found.ID != dm.ID {
		t.Errorf("GetDirectChannelByName: got %s, want %s", found.ID, dm.ID)
	}
}

// The team channel list is for browsing and joining. It used to include
// private channels, so any team member could read their names, headers and
// purposes without being in them.
func TestChannelStoreIntegration_GetChannelsForTeamListsOnlyOpen(t *testing.T) {
	ss := testStore(t)
	user, open := newTestChannelFixture(t, ss)

	if _, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID: open.TeamID, Name: "secret", DisplayName: "Secret",
		Type: model.ChannelPrivate, CreatorID: user.ID,
	}); err != nil {
		t.Fatalf("save private channel: %v", err)
	}

	channels, err := ss.Channel().GetChannelsForTeam(t.Context(), open.TeamID, 0, 50)
	if err != nil {
		t.Fatalf("GetChannelsForTeam: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != open.ID {
		t.Errorf("got %d channels, want only the open one", len(channels))
	}
}

// A deleted channel has no members, as far as access checks are concerned.
func TestChannelStoreIntegration_DeletedChannelHasNoMembers(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	if _, err := ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: channel.ID, UserID: user.ID}); err != nil {
		t.Fatalf("SaveMember: %v", err)
	}
	if err := ss.Channel().Delete(t.Context(), channel.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := ss.Channel().GetMember(t.Context(), channel.ID, user.ID); err == nil {
		t.Error("GetMember found a member of a deleted channel")
	}
	ids, err := ss.Channel().GetChannelIDsForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetChannelIDsForUser: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("GetChannelIDsForUser listed a deleted channel: %v", ids)
	}
}

func TestChannelStoreIntegration_TeamWideMembershipAndDeletion(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	otherTeam, err := ss.Team().Save(t.Context(), &model.Team{Name: "other-team", DisplayName: "Other", Type: model.TeamOpen, CreatorID: user.ID})
	if err != nil {
		t.Fatalf("save team: %v", err)
	}
	elsewhere, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID: otherTeam.ID, Name: "elsewhere", DisplayName: "Elsewhere", Type: model.ChannelOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	for _, id := range []string{channel.ID, elsewhere.ID} {
		if _, err = ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: id, UserID: user.ID}); err != nil {
			t.Fatalf("SaveMember: %v", err)
		}
	}

	removed, err := ss.Channel().RemoveMemberFromTeam(t.Context(), channel.TeamID, user.ID)
	if err != nil {
		t.Fatalf("RemoveMemberFromTeam: %v", err)
	}
	if len(removed) != 1 || removed[0] != channel.ID {
		t.Errorf("removed from %v, want only %s", removed, channel.ID)
	}
	if _, err = ss.Channel().GetMember(t.Context(), elsewhere.ID, user.ID); err != nil {
		t.Errorf("membership on another team was removed: %v", err)
	}

	deleted, err := ss.Channel().DeleteForTeam(t.Context(), otherTeam.ID, model.GetMillis())
	if err != nil {
		t.Fatalf("DeleteForTeam: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != elsewhere.ID {
		t.Errorf("deleted %v, want only %s", deleted, elsewhere.ID)
	}
	if _, err = ss.Channel().Get(t.Context(), channel.ID); err != nil {
		t.Errorf("a channel on another team was deleted: %v", err)
	}
}

func TestChannelStoreIntegration_MentionQueries(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("mentionee"))
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := ss.User().Save(t.Context(), newTestUser("outsider"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{user.ID, other.ID} {
		if _, err = ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: channel.ID, UserID: id}); err != nil {
			t.Fatal(err)
		}
	}

	all, err := ss.Channel().GetMemberIDs(t.Context(), channel.ID)
	if err != nil || len(all) != 2 {
		t.Fatalf("GetMemberIDs = %v, %v; want both members", all, err)
	}

	named, err := ss.Channel().GetMemberIDsByUsernames(t.Context(), channel.ID,
		[]string{"mentionee", "outsider", "nosuchuser"})
	if err != nil {
		t.Fatalf("GetMemberIDsByUsernames: %v", err)
	}
	if len(named) != 1 || named[0] != other.ID {
		t.Errorf("resolved %v, want only the member (%s), not %s", named, other.ID, outsider.ID)
	}

	if err = ss.Channel().IncrementMentionCounts(t.Context(), channel.ID, []string{other.ID, outsider.ID}); err != nil {
		t.Fatalf("IncrementMentionCounts: %v", err)
	}
	m, err := ss.Channel().GetMember(t.Context(), channel.ID, other.ID)
	if err != nil || m.MentionCount != 1 {
		t.Errorf("mention_count = %v (%v), want 1", m, err)
	}
}

func TestChannelStoreIntegration_AddTeamMembers(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("teammate"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{user.ID, other.ID} {
		if _, err = ss.Team().SaveMember(t.Context(), &model.TeamMember{TeamID: channel.TeamID, UserID: id}); err != nil {
			t.Fatal(err)
		}
	}
	// The creator is already in; only the teammate is new.
	if _, err = ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: channel.ID, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}

	added, err := ss.Channel().AddTeamMembers(t.Context(), channel.ID, channel.TeamID)
	if err != nil {
		t.Fatalf("AddTeamMembers: %v", err)
	}
	if len(added) != 1 || added[0] != other.ID {
		t.Errorf("added %v, want only the teammate %s", added, other.ID)
	}
	if _, err = ss.Channel().GetMember(t.Context(), channel.ID, other.ID); err != nil {
		t.Errorf("teammate is not a member: %v", err)
	}
}

// Direct channels are unique by name. Without the index two concurrent
// "open a DM" requests each created one and the conversation split.
func TestChannelStoreIntegration_DirectChannelNameIsUnique(t *testing.T) {
	ss := testStore(t)
	a, err := ss.User().Save(t.Context(), newTestUser("dmone"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ss.User().Save(t.Context(), newTestUser("dmtwo"))
	if err != nil {
		t.Fatal(err)
	}
	name := a.ID + "__" + b.ID
	dm := func() (*model.Channel, error) {
		return ss.Channel().SaveDirectChannel(t.Context(), &model.Channel{
			Name: name, DisplayName: "dm", Type: model.ChannelDirect, CreatorID: a.ID,
		}, []string{a.ID, b.ID})
	}
	if _, err = dm(); err != nil {
		t.Fatalf("first: %v", err)
	}
	var appErr *model.AppError
	if _, err = dm(); !errors.As(err, &appErr) || appErr.StatusCode != http.StatusConflict {
		t.Fatalf("second: got %v, want a 409 so the caller can fetch the existing channel", err)
	}
}

// GetMembersForUser returns one user's own member rows, for one team's live
// channels only: the counts behind their unread and mention badges.
func TestChannelStoreIntegration_GetMembersForUser(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	other, err := ss.User().Save(t.Context(), newTestUser("member3"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}
	gone, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID: channel.TeamID, Name: "gone", DisplayName: "Gone",
		Type: model.ChannelOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	otherTeam, err := ss.Team().Save(t.Context(), &model.Team{
		Name: "other-team", DisplayName: "Other", Type: model.TeamOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save team: %v", err)
	}
	elsewhere, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID: otherTeam.ID, Name: "elsewhere", DisplayName: "Elsewhere",
		Type: model.ChannelOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	for _, m := range []*model.ChannelMember{
		{ChannelID: channel.ID, UserID: user.ID, MentionCount: 2},
		{ChannelID: channel.ID, UserID: other.ID, MentionCount: 9},
		{ChannelID: gone.ID, UserID: user.ID},
		{ChannelID: elsewhere.ID, UserID: user.ID},
	} {
		if _, err = ss.Channel().SaveMember(t.Context(), m); err != nil {
			t.Fatalf("SaveMember: %v", err)
		}
	}
	if err = ss.Channel().Delete(t.Context(), gone.ID, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	members, err := ss.Channel().GetMembersForUser(t.Context(), user.ID, channel.TeamID)
	if err != nil {
		t.Fatalf("GetMembersForUser: %v", err)
	}
	if len(members) != 1 || members[0].ChannelID != channel.ID || members[0].UserID != user.ID {
		t.Fatalf("got %+v, want only %s's row in %s", members, user.ID, channel.ID)
	}
	if members[0].MentionCount != 2 {
		t.Errorf("MentionCount = %d, want 2", members[0].MentionCount)
	}
}
