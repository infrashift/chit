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
		if err = ss.Thread().SaveOrUpdate(t.Context(), thread); err != nil {
			t.Fatalf("SaveOrUpdate (reply %d): %v", i+1, err)
		}
		if err = ss.Thread().IncrementReplyCount(t.Context(), root.ID, reply.CreateAt, ru.ID); err != nil {
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

// The thread inbox lists only threads the user can still read. Following is
// not access: it used to keep showing a thread's root after the user left the
// channel, and kept showing deleted roots.
func TestThreadStoreIntegration_InboxRequiresAccess(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	if _, err := ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: channel.ID, UserID: user.ID}); err != nil {
		t.Fatalf("SaveMember: %v", err)
	}
	follow := func(content string, at int64) *model.Post {
		t.Helper()
		root := savePost(t, ss, channel.ID, user.ID, "", content, at)
		if err := ss.Thread().SaveOrUpdate(t.Context(), &model.Thread{PostID: root.ID, ChannelID: channel.ID, Participants: []string{}}); err != nil {
			t.Fatalf("SaveOrUpdate: %v", err)
		}
		if err := ss.Thread().SaveMembership(t.Context(), &model.ThreadMembership{PostID: root.ID, UserID: user.ID, Following: true}); err != nil {
			t.Fatalf("SaveMembership: %v", err)
		}
		return root
	}
	kept := follow("kept", 1000)
	deleted := follow("deleted", 2000)
	if err := ss.Post().Delete(t.Context(), deleted.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	inbox := func() []string {
		t.Helper()
		list, err := ss.Thread().GetThreadsForUser(t.Context(), user.ID, channel.TeamID, 0, 50)
		if err != nil {
			t.Fatalf("GetThreadsForUser: %v", err)
		}
		var ids []string
		for _, tr := range list.Threads {
			ids = append(ids, tr.Thread.PostID)
		}
		if int(list.Total) != len(ids) {
			t.Errorf("Total = %d but %d threads listed", list.Total, len(ids))
		}
		return ids
	}

	if got := inbox(); len(got) != 1 || got[0] != kept.ID {
		t.Fatalf("member inbox = %v, want only %s (deleted root hidden)", got, kept.ID)
	}

	if err := ss.Channel().RemoveMember(t.Context(), channel.ID, user.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if got := inbox(); len(got) != 0 {
		t.Fatalf("inbox after leaving the channel = %v, want empty", got)
	}
}

// Re-saving a membership keeps the later read mark, and DecrementReplyCount
// recomputes last_reply_at from the replies left.
func TestThreadStoreIntegration_MembershipAndDecrement(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	root := savePost(t, ss, channel.ID, user.ID, "", "root", 1000)
	if err := ss.Thread().SaveOrUpdate(t.Context(), &model.Thread{PostID: root.ID, ChannelID: channel.ID, Participants: []string{}}); err != nil {
		t.Fatalf("SaveOrUpdate: %v", err)
	}
	r1 := savePost(t, ss, channel.ID, user.ID, root.ID, "one", 2000)
	r2 := savePost(t, ss, channel.ID, user.ID, root.ID, "two", 3000)
	for _, r := range []*model.Post{r1, r2} {
		if err := ss.Thread().IncrementReplyCount(t.Context(), root.ID, r.CreateAt, user.ID); err != nil {
			t.Fatalf("IncrementReplyCount: %v", err)
		}
	}

	save := func(at int64) {
		t.Helper()
		if err := ss.Thread().SaveMembership(t.Context(), &model.ThreadMembership{
			PostID: root.ID, UserID: user.ID, Following: true, LastViewedAt: at,
		}); err != nil {
			t.Fatalf("SaveMembership: %v", err)
		}
	}
	save(3000)
	save(0)
	m, err := ss.Thread().GetMembership(t.Context(), root.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMembership: %v", err)
	}
	if m.LastViewedAt != 3000 {
		t.Errorf("last_viewed_at = %d, want 3000 kept", m.LastViewedAt)
	}

	if err = ss.Post().Delete(t.Context(), r2.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err = ss.Thread().DecrementReplyCount(t.Context(), root.ID); err != nil {
		t.Fatalf("DecrementReplyCount: %v", err)
	}
	th, err := ss.Thread().Get(t.Context(), root.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if th.ReplyCount != 1 || th.LastReplyAt != 2000 {
		t.Errorf("thread = %d replies, last at %d; want 1 and 2000", th.ReplyCount, th.LastReplyAt)
	}
}
