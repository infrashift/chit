//go:build integration

package sqlstore

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
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

// Search applies scope, author, terms and tags in the query, before LIMIT.
func TestPostStoreIntegration_Search(t *testing.T) {
	ss := testStore(t)
	for _, tc := range searchCases(seedSearch(t, ss)) {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ss.Post().Search(t.Context(), &tc.q)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			ids := make([]string, 0, len(got))
			for _, p := range got {
				ids = append(ids, p.ID)
			}
			if !slices.Equal(ids, tc.want) {
				t.Errorf("got %v, want %v", ids, tc.want)
			}
		})
	}
}

// searchFixture names what seedSearch saves.
type searchFixture struct {
	channel, other, a, b, c, pct, bug, urgent string
}

// searchCase is one Search query and the post IDs it returns, in order.
type searchCase struct {
	name string
	q    model.PostSearch
	want []string
}

// seedSearch saves posts by two authors in two channels, with tags on some
// of them.
func seedSearch(t *testing.T, ss *SqlStore) *searchFixture {
	t.Helper()
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("searchother"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}
	elsewhere, err := ss.Channel().Save(t.Context(), &model.Channel{
		TeamID: channel.TeamID, Name: "elsewhere", DisplayName: "Elsewhere",
		Type: model.ChannelOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	a := savePost(t, ss, channel.ID, user.ID, "", "needle one", 1000)
	b := savePost(t, ss, channel.ID, other.ID, "", "needle two", 2000)
	c := savePost(t, ss, channel.ID, user.ID, "", "needle three", 3000)
	savePost(t, ss, elsewhere.ID, user.ID, "", "needle out of scope", 4000)
	pct := savePost(t, ss, channel.ID, user.ID, "", "100% done", 5000)

	bug, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "bug", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: user.ID})
	if err != nil {
		t.Fatalf("save tag: %v", err)
	}
	urgent, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "urgent", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: user.ID})
	if err != nil {
		t.Fatalf("save tag: %v", err)
	}
	for _, pt := range [][2]string{{a.ID, bug.ID}, {a.ID, urgent.ID}, {c.ID, bug.ID}} {
		if err = ss.Tag().AddTagToPost(t.Context(), pt[0], pt[1], user.ID); err != nil {
			t.Fatalf("AddTagToPost: %v", err)
		}
	}
	return &searchFixture{channel: channel.ID, other: other.ID, a: a.ID, b: b.ID, c: c.ID,
		pct: pct.ID, bug: bug.ID, urgent: urgent.ID}
}

func searchCases(f *searchFixture) []searchCase {
	scope := []string{f.channel}
	return []searchCase{
		{"terms, newest first", model.PostSearch{Terms: "needle", ChannelIDs: scope, PerPage: 10}, []string{f.c, f.b, f.a}},
		{"pagination is after scoping", model.PostSearch{Terms: "needle", ChannelIDs: scope, Page: 1, PerPage: 2}, []string{f.a}},
		{"author", model.PostSearch{Terms: "needle", ChannelIDs: scope, AuthorID: f.other, PerPage: 10}, []string{f.b}},
		{"one tag", model.PostSearch{TagIDs: []string{f.bug}, ChannelIDs: scope, PerPage: 10}, []string{f.c, f.a}},
		{"every tag must match", model.PostSearch{TagIDs: []string{f.bug, f.urgent}, ChannelIDs: scope, PerPage: 10}, []string{f.a}},
		{"terms and tags", model.PostSearch{Terms: "three", TagIDs: []string{f.bug}, ChannelIDs: scope, PerPage: 10}, []string{f.c}},
		{"% is literal", model.PostSearch{Terms: "0%", ChannelIDs: scope, PerPage: 10}, []string{f.pct}},
		{"empty scope matches nothing", model.PostSearch{Terms: "needle", PerPage: 10}, nil},
	}
}

func TestPostStoreIntegration_GetByIDsAndBadIDs(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	kept := savePost(t, ss, channel.ID, user.ID, "", "kept", 1000)
	gone := savePost(t, ss, channel.ID, user.ID, "", "gone", 2000)
	if err := ss.Post().Delete(t.Context(), gone.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := ss.Post().GetByIDs(t.Context(), []string{kept.ID, gone.ID, model.NewID()})
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != kept.ID {
		t.Errorf("GetByIDs returned %d posts, want only the live one", len(got))
	}

	var appErr *model.AppError
	if _, err = ss.Post().Get(t.Context(), "not-a-uuid"); !errors.As(err, &appErr) || appErr.StatusCode != http.StatusNotFound {
		t.Errorf("Get(not-a-uuid) = %v, want a 404 AppError rather than a uuid cast failure", err)
	}
}

// GetPostsSince pages by (update_at, id), so posts sharing an update_at are
// split across pages without being skipped or repeated, and stops at until.
func TestPostStoreIntegration_GetPostsSinceKeyset(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	var want []string
	for i := range 5 {
		p := savePost(t, ss, channel.ID, user.ID, "", fmt.Sprintf("tie %d", i), 1000)
		want = append(want, p.ID)
	}
	late := savePost(t, ss, channel.ID, user.ID, "", "late", 1000)
	// Give every post the same update_at, and one a later one.
	if _, err := ss.pool.Exec(t.Context(), `UPDATE posts SET update_at = 500 WHERE channel_id = $1`, channel.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.pool.Exec(t.Context(), `UPDATE posts SET update_at = 900 WHERE id = $1`, late.ID); err != nil {
		t.Fatal(err)
	}
	slices.Sort(want)

	var got []string
	var cursor model.PostCursor
	for {
		page, err := ss.Post().GetPostsSince(t.Context(), cursor, 800, 2)
		if err != nil {
			t.Fatalf("GetPostsSince: %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, p := range page {
			got = append(got, p.ID)
		}
		last := page[len(page)-1]
		cursor = model.PostCursor{UpdateAt: last.UpdateAt, ID: last.ID}
	}
	if !slices.Equal(got, want) {
		t.Errorf("paged %v, want %v (each tied post once, the post past until excluded)", got, want)
	}
}
