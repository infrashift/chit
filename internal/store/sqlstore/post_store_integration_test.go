//go:build integration

package sqlstore

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestPostStoreIntegration_GetPostsForChannelSince(t *testing.T) {
	ss := testStore(t)

	user, channel := newTestChannelFixture(t, ss)

	base := model.GetMillis()
	old := savePost(t, ss, channel.ID, user.ID, "", "old post", base-10_000)
	mid := savePost(t, ss, channel.ID, user.ID, "", "mid post", base-5_000)
	newest := savePost(t, ss, channel.ID, user.ID, "", "new post", base-1_000)

	// Without Since: all posts, newest first.
	all, err := ss.Post().GetPostsForChannel(t.Context(), channel.ID, model.GetPostsOptions{PerPage: 60})
	if err != nil {
		t.Fatalf("GetPostsForChannel: %v", err)
	}
	if len(all.Order) != 3 {
		t.Fatalf("without Since: got %d posts, want 3", len(all.Order))
	}
	if all.Order[0].ID != newest.ID {
		t.Errorf("without Since: first post is %q, want newest %q", all.Order[0].ID, newest.ID)
	}

	// With Since: only posts strictly after the timestamp, oldest first.
	polled, err := ss.Post().GetPostsForChannel(t.Context(), channel.ID, model.GetPostsOptions{
		PerPage: 60,
		Since:   old.CreateAt,
	})
	if err != nil {
		t.Fatalf("GetPostsForChannel with Since: %v", err)
	}
	if len(polled.Order) != 2 {
		t.Fatalf("with Since: got %d posts, want 2", len(polled.Order))
	}
	if polled.Order[0].ID != mid.ID || polled.Order[1].ID != newest.ID {
		t.Errorf("with Since: got order [%q, %q], want oldest-first [%q, %q]",
			polled.Order[0].ID, polled.Order[1].ID, mid.ID, newest.ID)
	}

	// Since equal to the newest post's timestamp returns nothing (strictly after).
	empty, err := ss.Post().GetPostsForChannel(t.Context(), channel.ID, model.GetPostsOptions{
		PerPage: 60,
		Since:   newest.CreateAt,
	})
	if err != nil {
		t.Fatalf("GetPostsForChannel with Since=newest: %v", err)
	}
	if len(empty.Order) != 0 {
		t.Errorf("with Since=newest: got %d posts, want 0", len(empty.Order))
	}
}

func TestPostStoreIntegration_GetPostsForChannelClampsPagination(t *testing.T) {
	ss := testStore(t)

	user, channel := newTestChannelFixture(t, ss)
	savePost(t, ss, channel.ID, user.ID, "", "a post", 0)

	// Negative page / zero per_page must not produce SQL errors.
	list, err := ss.Post().GetPostsForChannel(t.Context(), channel.ID, model.GetPostsOptions{Page: -3, PerPage: 0})
	if err != nil {
		t.Fatalf("GetPostsForChannel with negative page: %v", err)
	}
	if len(list.Order) != 1 {
		t.Errorf("got %d posts, want 1", len(list.Order))
	}
}
