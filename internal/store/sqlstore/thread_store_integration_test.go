//go:build integration

package sqlstore

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// newTestChannelFixture creates a user, team, and open channel for tests that
// need posts. Shared by thread and post integration tests.
func newTestChannelFixture(t *testing.T, ss *SqlStore) (*model.User, *model.Channel) {
	t.Helper()

	user, err := ss.User().Save(t.Context(), newTestUser("threadtester"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}

	team, err := ss.Team().Save(t.Context(), &model.Team{
		Name:        "test-team",
		DisplayName: "Test Team",
		Type:        model.TeamOpen,
		CreatorID:   user.ID,
	})
	if err != nil {
		t.Fatalf("save team: %v", err)
	}

	channel, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID:      team.ID,
		Name:        "test-channel",
		DisplayName: "Test Channel",
		Type:        model.ChannelOpen,
		CreatorID:   user.ID,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	return user, channel
}

func savePost(t *testing.T, ss *SqlStore, channelID, userID, rootID, content string, createAt int64) *model.Post {
	t.Helper()
	post := &model.Post{
		ChannelID: channelID,
		UserID:    userID,
		RootID:    rootID,
		Content:   content,
		CreateAt:  createAt,
	}
	saved, err := ss.Post().Save(t.Context(), post)
	if err != nil {
		t.Fatalf("save post: %v", err)
	}
	return saved
}

// Regression test: SaveOrUpdate must not reset counters of an existing thread.
// Before the fix, every reply reset reply_count to 0 (then incremented to 1)
// and clobbered participants.
func TestThreadStoreIntegration_ReplyCountAccumulates(t *testing.T) {
	ss := testStore(t)

	user, channel := newTestChannelFixture(t, ss)
	replier, err := ss.User().Save(t.Context(), newTestUser("replier"))
	if err != nil {
		t.Fatalf("save replier: %v", err)
	}

	root := savePost(t, ss, channel.ID, user.ID, "", "root post", 0)

	// Simulate app.handleThreadReply for three replies from two users.
	repliers := []*model.User{user, replier, replier}
	for i, ru := range repliers {
		reply := savePost(t, ss, channel.ID, ru.ID, root.ID, "reply", 0)

		thread := &model.Thread{
			PostID:       root.ID,
			ChannelID:    channel.ID,
			ReplyCount:   0,
			LastReplyAt:  0,
			Participants: []string{},
		}
		if err := ss.Thread().SaveOrUpdate(t.Context(), thread); err != nil {
			t.Fatalf("SaveOrUpdate (reply %d): %v", i+1, err)
		}
		if err := ss.Thread().IncrementReplyCount(t.Context(), root.ID, reply.CreateAt, ru.ID); err != nil {
			t.Fatalf("IncrementReplyCount (reply %d): %v", i+1, err)
		}
	}

	got, err := ss.Thread().Get(t.Context(), root.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ReplyCount != 3 {
		t.Errorf("ReplyCount: got %d, want 3", got.ReplyCount)
	}
	if got.LastReplyAt == 0 {
		t.Error("LastReplyAt: got 0, want the last reply's timestamp")
	}
	if len(got.Participants) != 2 {
		t.Errorf("Participants: got %v (len %d), want both repliers (len 2)", got.Participants, len(got.Participants))
	}
}

func TestThreadStoreIntegration_SaveOrUpdateIsIdempotent(t *testing.T) {
	ss := testStore(t)

	user, channel := newTestChannelFixture(t, ss)
	root := savePost(t, ss, channel.ID, user.ID, "", "root post", 0)

	thread := &model.Thread{
		PostID:       root.ID,
		ChannelID:    channel.ID,
		Participants: []string{},
	}
	if err := ss.Thread().SaveOrUpdate(t.Context(), thread); err != nil {
		t.Fatalf("SaveOrUpdate: %v", err)
	}
	if err := ss.Thread().IncrementReplyCount(t.Context(), root.ID, model.GetMillis(), user.ID); err != nil {
		t.Fatalf("IncrementReplyCount: %v", err)
	}

	// A second SaveOrUpdate (as happens on every reply) must not change state.
	if err := ss.Thread().SaveOrUpdate(t.Context(), thread); err != nil {
		t.Fatalf("SaveOrUpdate (second): %v", err)
	}

	got, err := ss.Thread().Get(t.Context(), root.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ReplyCount != 1 {
		t.Errorf("ReplyCount after redundant SaveOrUpdate: got %d, want 1", got.ReplyCount)
	}
	if len(got.Participants) != 1 {
		t.Errorf("Participants after redundant SaveOrUpdate: got %v, want 1 entry", got.Participants)
	}
}
