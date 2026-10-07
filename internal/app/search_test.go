package app

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// newSearchFixture puts "searcher" in the general channel only. other is a
// channel in the same team the searcher cannot read.
func newSearchFixture(t *testing.T) (f *fixture, other *model.Channel) {
	t.Helper()
	f = newFixture(t)
	f.join(f.channel, "searcher")
	other = f.privateChannel("other")
	f.join(other, "outsider")
	return f, other
}

func (f *fixture) search(channelID, terms string, tagIDs []string) []*model.Post {
	f.t.Helper()
	res, err := f.app.SearchPosts(f.t.Context(), channelID, f.user("searcher").ID, terms, tagIDs, 0, 60)
	if err != nil {
		f.t.Fatalf("SearchPosts: %v", err)
	}
	return res.Order
}

func TestSearchPosts_SQLWhenZincIsNotConfigured(t *testing.T) {
	f, _ := newSearchFixture(t)
	f.post(f.channel, "searcher", "hello world", 1)
	f.post(f.channel, "searcher", "hello again", 2)
	f.post(f.channel, "searcher", "goodbye", 3)

	if got := f.search(f.channel.ID, "hello", nil); len(got) != 2 {
		t.Errorf("got %d posts, want the 2 that match", len(got))
	}
}

func TestSearchPosts_NoQueryNoTags_ReturnsEmpty(t *testing.T) {
	f, _ := newSearchFixture(t)
	f.post(f.channel, "searcher", "anything", 1)

	if got := f.search(f.channel.ID, "", nil); len(got) != 0 {
		t.Errorf("got %d posts, want none", len(got))
	}
}

func TestSearchPosts_Tags(t *testing.T) {
	f, _ := newSearchFixture(t)
	tag := &model.Tag{ID: model.NewID(), Name: "bug"}
	f.store.Tags.Seed(tag)
	tagged := f.post(f.channel, "searcher", "tagged search", 1)
	f.post(f.channel, "searcher", "untagged search", 2)
	f.store.Tags.SeedPostTag(tagged.ID, tag.ID)

	t.Run("tags alone", func(t *testing.T) {
		if got := f.search(f.channel.ID, "", []string{tag.ID}); len(got) != 1 || got[0].ID != tagged.ID {
			t.Errorf("got %v, want only the tagged post", got)
		}
	})
	t.Run("tags and terms", func(t *testing.T) {
		if got := f.search(f.channel.ID, "search", []string{tag.ID}); len(got) != 1 || got[0].ID != tagged.ID {
			t.Errorf("got %v, want only the tagged post", got)
		}
	})
}

// Membership is not scope. A search "in this channel" was returning hits from
// every other channel the caller belonged to, because the index is queried
// without a channel filter and only membership was checked afterwards.
func TestSearchPosts_ScopesToTheRequestedChannel(t *testing.T) {
	f, _ := newSearchFixture(t)
	elsewhere := &model.Channel{
		ID: model.NewID(), TeamID: f.team.ID, Name: "random", DisplayName: "Random",
		Type: model.ChannelOpen, CreateAt: 1, UpdateAt: 1,
	}
	f.store.Channels.Seed(elsewhere)
	f.join(elsewhere, "searcher")
	here := f.post(f.channel, "searcher", "needle in this channel", 1)
	f.post(elsewhere, "searcher", "needle in another channel", 2)

	got := f.search(f.channel.ID, "needle", nil)
	if len(got) != 1 || got[0].ID != here.ID {
		t.Errorf("got %v, want just the post in the requested channel", got)
	}
}

// An unscoped search spans every channel the caller can read, and only
// those.
func TestSearchPosts_UnscopedIsStillLimitedByMembership(t *testing.T) {
	f, other := newSearchFixture(t)
	readable := f.post(f.channel, "searcher", "needle one", 1)
	f.post(other, "outsider", "needle two", 2)

	got := f.search("", "needle", nil)
	if len(got) != 1 || got[0].ID != readable.ID {
		t.Errorf("got %v, want only the readable post", got)
	}
}
