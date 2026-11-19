//go:build integration

package sqlstore

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestTagStoreIntegration_TagsAndAssociations(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	post1 := savePost(t, ss, channel.ID, user.ID, "", "tagged one", 0)
	post2 := savePost(t, ss, channel.ID, user.ID, "", "tagged two", 0)

	tag, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "important", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: user.ID})
	if err != nil {
		t.Fatalf("Save tag: %v", err)
	}

	if err = ss.Tag().AddTagToPost(t.Context(), post1.ID, tag.ID, user.ID); err != nil {
		t.Fatalf("AddTagToPost: %v", err)
	}
	var appliedBy string
	if err = ss.pool.QueryRow(t.Context(), `SELECT applied_by FROM message_tags WHERE message_id = $1`, post1.ID).Scan(&appliedBy); err != nil || appliedBy != user.ID {
		t.Errorf("applied_by = %q, %v; want %s", appliedBy, err, user.ID)
	}

	tags, err := ss.Tag().GetTagsForPost(t.Context(), post1.ID)
	if err != nil {
		t.Fatalf("GetTagsForPost: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != tag.ID || tags[0].ChannelID != channel.ID || tags[0].Scope != model.TagScopeChannel {
		t.Errorf("GetTagsForPost: got %+v", tags)
	}

	found, err := ss.Post().Search(t.Context(), &model.PostSearch{
		TagIDs: []string{tag.ID}, ChannelIDs: []string{post1.ChannelID}, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("Search by tag: %v", err)
	}
	if len(found) != 1 || found[0].ID != post1.ID {
		t.Errorf("Search by tag: got %d posts, want only %s", len(found), post1.ID)
	}

	filtered, err := ss.Tag().FilterPostIDsByTags(t.Context(), []string{post1.ID, post2.ID}, []string{tag.ID})
	if err != nil {
		t.Fatalf("FilterPostIDsByTags: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != post1.ID {
		t.Errorf("FilterPostIDsByTags: got %v, want [%s]", filtered, post1.ID)
	}

	if err = ss.Tag().RemoveTagFromPost(t.Context(), post1.ID, tag.ID); err != nil {
		t.Fatalf("RemoveTagFromPost: %v", err)
	}
	tags, err = ss.Tag().GetTagsForPost(t.Context(), post1.ID)
	if err != nil {
		t.Fatalf("GetTagsForPost after remove: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("GetTagsForPost after remove: got %v, want empty", tags)
	}
}

// GetVisible is the list query behind GET /tags: the reader's private tags
// and the channel tags of the reader's channels, never another owner's
// private tag.
func TestTagStoreIntegration_VisibilityAndUniqueness(t *testing.T) {
	ss := testStore(t)
	alice, channel := newTestChannelFixture(t, ss)
	bob, err := ss.User().Save(t.Context(), newTestUser("bob"))
	if err != nil {
		t.Fatal(err)
	}
	save := func(tag *model.Tag) *model.Tag {
		t.Helper()
		saved, saveErr := ss.Tag().Save(t.Context(), tag)
		if saveErr != nil {
			t.Fatalf("save %s: %v", tag.Name, saveErr)
		}
		return saved
	}
	alicePrivate := save(&model.Tag{Name: "Mine", Scope: model.TagScopePrivate, OwnerID: alice.ID})
	bobPrivate := save(&model.Tag{Name: "mine", Scope: model.TagScopePrivate, OwnerID: bob.ID}) // same key, other owner
	shared := save(&model.Tag{Name: "Shared", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: bob.ID})

	var appErr *model.AppError
	if _, err = ss.Tag().Save(t.Context(), &model.Tag{Name: "MINE", Scope: model.TagScopePrivate, OwnerID: alice.ID}); !errors.As(err, &appErr) || appErr.StatusCode != http.StatusConflict {
		t.Errorf("a second private tag with one key for one owner: %v, want 409", err)
	}
	if _, err = ss.Tag().Save(t.Context(), &model.Tag{Name: "ｓｈａｒｅｄ", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: alice.ID}); !errors.As(err, &appErr) || appErr.StatusCode != http.StatusConflict {
		t.Errorf("a second channel tag with one (NFKC) key: %v, want 409", err)
	}

	ids := func(tags []*model.Tag) []string {
		out := []string{}
		for _, tag := range tags {
			out = append(out, tag.ID)
		}
		return out
	}
	aliceSees, err := ss.Tag().GetVisible(t.Context(), alice.ID, []string{channel.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(aliceSees); !slices.Contains(got, alicePrivate.ID) || !slices.Contains(got, shared.ID) || slices.Contains(got, bobPrivate.ID) {
		t.Errorf("alice sees %v; want her private tag and the channel tag, not bob's", got)
	}
	outsider, err := ss.Tag().GetVisible(t.Context(), alice.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(outsider); slices.Contains(got, shared.ID) {
		t.Errorf("without the channel alice still sees its tag: %v", got)
	}

	got, err := ss.Tag().GetByKey(t.Context(), model.TagScopePrivate, bob.ID, "", model.TagNameKey("MINE"))
	if err != nil || got.ID != bobPrivate.ID {
		t.Errorf("GetByKey = %v, %v; want bob's tag", got, err)
	}
}

func TestTagStoreIntegration_ChannelTagsFilterAndDeleteByOwner(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)
	other, err := ss.User().Save(t.Context(), newTestUser("other"))
	if err != nil {
		t.Fatal(err)
	}
	post := savePost(t, ss, channel.ID, user.ID, "", "p", 1000)
	a, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "a", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "b", Scope: model.TagScopeChannel, ChannelID: channel.ID, OwnerID: other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err = ss.Tag().AddTagToChannel(t.Context(), channel.ID, a.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	if err = ss.Tag().AddTagToPost(t.Context(), post.ID, b.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	match, err := ss.Tag().FilterChannelIDsByTags(t.Context(), []string{channel.ID}, []string{a.ID, b.ID})
	if err != nil || len(match) != 1 {
		t.Fatalf("a on the channel and b on a post: %v, %v; want the channel", match, err)
	}
	if err = ss.Post().Delete(t.Context(), post.ID, 5000); err != nil {
		t.Fatal(err)
	}
	if match, err = ss.Tag().FilterChannelIDsByTags(t.Context(), []string{channel.ID}, []string{a.ID, b.ID}); err != nil {
		t.Fatalf("FilterChannelIDsByTags after delete: %v", err)
	}
	if len(match) != 0 {
		t.Errorf("a deleted post still matched: %v", match)
	}

	// other's tag goes; user's tag that other applied stays, anonymously.
	if err = ss.Tag().DeleteByOwner(t.Context(), other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = ss.Tag().Get(t.Context(), b.ID); err == nil {
		t.Error("the owner's tag survived")
	}
	var appliedBy *string
	if err = ss.pool.QueryRow(t.Context(), `SELECT applied_by FROM channel_tags WHERE tag_id = $1`, a.ID).Scan(&appliedBy); err != nil || appliedBy != nil {
		t.Errorf("applied_by after the applier's deletion = %v, %v; want NULL", appliedBy, err)
	}
}
