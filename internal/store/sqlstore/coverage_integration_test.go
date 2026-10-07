//go:build integration

package sqlstore

import (
	"errors"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// Store methods that the app relies on and nothing else exercised against a
// real database.

func TestPostStoreIntegration_UpdatePinDeleteAndThreadOrder(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	root := savePost(t, ss, channel.ID, user.ID, "", "root", 1000)
	late := savePost(t, ss, channel.ID, user.ID, root.ID, "late reply", 3000)
	early := savePost(t, ss, channel.ID, user.ID, root.ID, "early reply", 2000)

	thread, err := ss.Post().GetPostsForThread(t.Context(), root.ID)
	if err != nil {
		t.Fatalf("GetPostsForThread: %v", err)
	}
	if got := thread.Order; len(got) != 3 || got[0].ID != root.ID || got[1].ID != early.ID || got[2].ID != late.ID {
		t.Fatalf("thread not oldest-first from the root: %v", got)
	}

	root.Content = "edited"
	root.Props = map[string]any{"k": "v"}
	if _, err = ss.Post().Update(t.Context(), root); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := ss.Post().Get(t.Context(), root.ID)
	if err != nil || got.Content != "edited" || got.Props["k"] != "v" || got.EditAt == 0 {
		t.Fatalf("after Update: %+v, %v", got, err)
	}

	if err = ss.Post().SetPinned(t.Context(), root.ID, true); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	pinned, err := ss.Post().GetPinnedPosts(t.Context(), channel.ID)
	if err != nil || len(pinned.Order) != 1 || pinned.Order[0].ID != root.ID {
		t.Fatalf("GetPinnedPosts = %v, %v", pinned, err)
	}

	if err = ss.Post().Delete(t.Context(), late.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err = ss.Post().Get(t.Context(), late.ID); err == nil {
		t.Error("a deleted post is still readable")
	}
	if err = ss.Post().Delete(t.Context(), late.ID, model.GetMillis()); err == nil {
		t.Error("deleting a deleted post should be a not-found")
	}
}

func TestChannelStoreIntegration_UpdateLookupAndReadState(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	if _, err := ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: channel.ID, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}

	channel.Header = "new header"
	if _, err := ss.Channel().Update(t.Context(), channel); err != nil {
		t.Fatalf("Update: %v", err)
	}
	byName, err := ss.Channel().GetByName(t.Context(), channel.TeamID, channel.Name)
	if err != nil || byName.ID != channel.ID || byName.Header != "new header" {
		t.Fatalf("GetByName = %+v, %v", byName, err)
	}

	// Two posts and a mention, then a view: msg_count catches up, mentions clear.
	for _, at := range []int64{1000, 2000} {
		if err = ss.Channel().IncrementMsgCount(t.Context(), channel.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	if err = ss.Channel().IncrementMentionCounts(t.Context(), channel.ID, []string{user.ID}); err != nil {
		t.Fatal(err)
	}
	if err = ss.Channel().UpdateLastViewedAt(t.Context(), channel.ID, user.ID, 5000); err != nil {
		t.Fatalf("UpdateLastViewedAt: %v", err)
	}
	m, err := ss.Channel().GetMember(t.Context(), channel.ID, user.ID)
	if err != nil || m.LastViewedAt != 5000 || m.MsgCount != 2 || m.MentionCount != 0 {
		t.Fatalf("member after view = %+v, %v; want viewed at 5000, msg_count 2, no mentions", m, err)
	}
}

func TestChannelStoreIntegration_GetDirectChannelsForUser(t *testing.T) {
	ss := testStore(t)
	a, err := ss.User().Save(t.Context(), newTestUser("dmlista"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ss.User().Save(t.Context(), newTestUser("dmlistb"))
	if err != nil {
		t.Fatal(err)
	}
	dm, err := ss.Channel().SaveDirectChannel(t.Context(), &model.Channel{
		Name: a.ID + "__" + b.ID, DisplayName: "dm", Type: model.ChannelDirect, CreatorID: a.ID,
	}, []string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	// A team channel the user is in must not appear among their DMs.
	_, team := newTestChannelFixture(t, ss)
	if _, err = ss.Channel().SaveMember(t.Context(), &model.ChannelMember{ChannelID: team.ID, UserID: a.ID}); err != nil {
		t.Fatal(err)
	}

	got, err := ss.Channel().GetDirectChannelsForUser(t.Context(), a.ID)
	if err != nil || len(got) != 1 || got[0].ID != dm.ID {
		t.Fatalf("GetDirectChannelsForUser = %v, %v; want only the DM", got, err)
	}
}

func TestThreadStoreIntegration_MarkAsReadAndUpdateMembership(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	root := savePost(t, ss, channel.ID, user.ID, "", "root", 1000)
	if err := ss.Thread().SaveOrUpdate(t.Context(), &model.Thread{PostID: root.ID, ChannelID: channel.ID, Participants: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := ss.Thread().MarkAsRead(t.Context(), root.ID, user.ID, 1); err == nil {
		t.Error("MarkAsRead without a membership should be a not-found")
	}
	if err := ss.Thread().SaveMembership(t.Context(), &model.ThreadMembership{PostID: root.ID, UserID: user.ID, Following: true}); err != nil {
		t.Fatal(err)
	}
	if err := ss.Thread().IncrementMentionCounts(t.Context(), root.ID, []string{user.ID}); err != nil {
		t.Fatal(err)
	}
	if err := ss.Thread().MarkAsRead(t.Context(), root.ID, user.ID, 4000); err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}
	m, err := ss.Thread().GetMembership(t.Context(), root.ID, user.ID)
	if err != nil || m.LastViewedAt != 4000 || m.UnreadMentionCount != 0 {
		t.Fatalf("after MarkAsRead: %+v, %v", m, err)
	}

	m.Following = false
	if err = ss.Thread().UpdateMembership(t.Context(), m); err != nil {
		t.Fatalf("UpdateMembership: %v", err)
	}
	if m, _ = ss.Thread().GetMembership(t.Context(), root.ID, user.ID); m.Following {
		t.Error("unfollow did not persist")
	}
}

func TestTagStoreIntegration_GetTagsForPosts(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	p1 := savePost(t, ss, channel.ID, user.ID, "", "one", 1000)
	p2 := savePost(t, ss, channel.ID, user.ID, "", "two", 2000)
	bug, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "bulkbug"})
	if err != nil {
		t.Fatal(err)
	}
	if err = ss.Tag().AddTagToPost(t.Context(), p1.ID, bug.ID); err != nil {
		t.Fatal(err)
	}

	got, err := ss.Tag().GetTagsForPosts(t.Context(), []string{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("GetTagsForPosts: %v", err)
	}
	if len(got[p1.ID]) != 1 || got[p1.ID][0].Name != "bulkbug" || len(got[p2.ID]) != 0 {
		t.Fatalf("GetTagsForPosts = %v; want only p1 tagged", got)
	}

	again, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "bulkbug"})
	if err != nil || again.ID != bug.ID {
		t.Fatalf("re-saving a tag name gave %v, %v; want the existing tag", again, err)
	}
}

// A malformed id is a 404 from every lookup, not a uuid cast failure.
func TestStoreIntegration_MalformedIDsAreNotFound(t *testing.T) {
	ss := testStore(t)
	valid := model.NewID()
	lookups := map[string]func() error{
		"channel":        func() error { _, err := ss.Channel().Get(t.Context(), "abc"); return err },
		"channel member": func() error { _, err := ss.Channel().GetMember(t.Context(), "abc", valid); return err },
		"team":           func() error { _, err := ss.Team().Get(t.Context(), "abc"); return err },
		"team member":    func() error { _, err := ss.Team().GetMember(t.Context(), valid, "abc"); return err },
	}
	for name, lookup := range lookups {
		var appErr *model.AppError
		if err := lookup(); !errors.As(err, &appErr) || appErr.StatusCode != http.StatusNotFound {
			t.Errorf("%s: got %v, want a 404 AppError", name, err)
		}
	}
}
