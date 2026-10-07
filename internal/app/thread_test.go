package app

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// newThreadFixture puts alice and bob in the general channel, with a root
// post by alice.
func newThreadFixture(t *testing.T) (f *fixture, root *model.Post) {
	t.Helper()
	f = newFixture(t)
	f.join(f.channel, "alice", "bob")
	root = f.post(f.channel, "alice", "root", 1000)
	return f, root
}

func (f *fixture) reply(root *model.Post, author string) *model.Post {
	f.t.Helper()
	p, err := f.app.CreatePost(f.t.Context(), &model.Post{
		ChannelID: root.ChannelID, RootID: root.ID, UserID: f.user(author).ID, Content: "reply",
	})
	if err != nil {
		f.t.Fatalf("reply: %v", err)
	}
	return p
}

func (f *fixture) membership(root *model.Post, name string) *model.ThreadMembership {
	f.t.Helper()
	m, err := f.store.Threads.GetMembership(f.t.Context(), root.ID, f.user(name).ID)
	if err != nil {
		f.t.Fatalf("GetMembership(%s): %v", name, err)
	}
	return m
}

// Every reply used to reset the replier's last_viewed_at to 0, so the thread
// stayed unread for whoever was most active in it. (That the upsert never
// moves the mark backwards is covered against Postgres in sqlstore.)
func TestReply_ReplierHasReadUpToTheirReply(t *testing.T) {
	f, root := newThreadFixture(t)
	for range 2 {
		r := f.reply(root, "bob")
		if m := f.membership(root, "bob"); !m.Following || m.LastViewedAt != r.CreateAt {
			t.Fatalf("after replying: %+v, want following and read up to the reply at %d", m, r.CreateAt)
		}
	}
}

func TestReply_RootAuthorFollowsTheirThread(t *testing.T) {
	t.Run("from the first reply", func(t *testing.T) {
		f, root := newThreadFixture(t)
		f.reply(root, "bob")
		if m := f.membership(root, "alice"); !m.Following {
			t.Fatal("the root author does not follow their own thread")
		}
	})
	t.Run("unless they unfollowed", func(t *testing.T) {
		f, root := newThreadFixture(t)
		f.reply(root, "bob")
		if err := f.app.UpdateThreadFollowing(t.Context(), root.ID, f.user("alice").ID, false); err != nil {
			t.Fatalf("unfollow: %v", err)
		}
		f.reply(root, "bob")
		if m := f.membership(root, "alice"); m.Following {
			t.Fatal("a later reply re-followed an author who had unfollowed")
		}
	})
}

func TestDeleteReply_LowersTheReplyCount(t *testing.T) {
	f, root := newThreadFixture(t)
	f.reply(root, "bob")
	r2 := f.reply(root, "bob")

	if err := f.app.DeletePost(t.Context(), r2.ID, f.user("bob").ID); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
	th, err := f.store.Threads.Get(t.Context(), root.ID)
	if err != nil {
		t.Fatalf("Get thread: %v", err)
	}
	if th.ReplyCount != 1 {
		t.Fatalf("reply_count = %d after deleting one of two replies, want 1", th.ReplyCount)
	}
}
