//go:build integration

package sqlstore

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestTagStoreIntegration_TagsAndAssociations(t *testing.T) {
	ss := testStore(t)
	user, channel := newTestChannelFixture(t, ss)

	post1 := savePost(t, ss, channel.ID, user.ID, "", "tagged one", 0)
	post2 := savePost(t, ss, channel.ID, user.ID, "", "tagged two", 0)

	tag, err := ss.Tag().Save(t.Context(), &model.Tag{Name: "important"})
	if err != nil {
		t.Fatalf("Save tag: %v", err)
	}

	all, err := ss.Tag().GetAll(t.Context())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 || all[0].Name != "important" {
		t.Errorf("GetAll: got %v", all)
	}

	if err = ss.Tag().AddTagToPost(t.Context(), post1.ID, tag.ID); err != nil {
		t.Fatalf("AddTagToPost: %v", err)
	}

	tags, err := ss.Tag().GetTagsForPost(t.Context(), post1.ID)
	if err != nil {
		t.Fatalf("GetTagsForPost: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != tag.ID {
		t.Errorf("GetTagsForPost: got %v", tags)
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
