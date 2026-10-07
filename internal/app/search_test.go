package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (f *fixture) search(req *SearchRequest) []*model.Post {
	f.t.Helper()
	req.UserID = f.user("searcher").ID
	if req.PerPage == 0 {
		req.PerPage = 60
	}
	res, err := f.app.SearchPosts(f.t.Context(), req)
	if err != nil {
		f.t.Fatalf("SearchPosts: %v", err)
	}
	return res.Order
}

func postIDs(posts []*model.Post) []string {
	ids := make([]string, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestSearchPosts_SQLWhenZincIsNotConfigured(t *testing.T) {
	f, _ := newSearchFixture(t)
	f.post(f.channel, "searcher", "hello world", 1)
	f.post(f.channel, "searcher", "hello again", 2)
	f.post(f.channel, "searcher", "goodbye", 3)

	if got := f.search(&SearchRequest{ChannelID: f.channel.ID, Terms: "hello"}); len(got) != 2 {
		t.Errorf("got %d posts, want the 2 that match", len(got))
	}
}

func TestSearchPosts_NoQueryNoTags_ReturnsEmpty(t *testing.T) {
	f, _ := newSearchFixture(t)
	f.post(f.channel, "searcher", "anything", 1)

	if got := f.search(&SearchRequest{ChannelID: f.channel.ID}); got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty, non-nil list", got)
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
		if got := f.search(&SearchRequest{TagIDs: []string{tag.ID}}); len(got) != 1 || got[0].ID != tagged.ID {
			t.Errorf("got %v, want only the tagged post", postIDs(got))
		}
	})
	t.Run("tags and terms", func(t *testing.T) {
		if got := f.search(&SearchRequest{Terms: "search", TagIDs: []string{tag.ID}}); len(got) != 1 || got[0].ID != tagged.ID {
			t.Errorf("got %v, want only the tagged post", postIDs(got))
		}
	})
	// The SQL matched HAVING COUNT(DISTINCT tag_id) = len(tagIDs), so a
	// repeated tag id could never match anything.
	t.Run("a repeated tag id", func(t *testing.T) {
		if got := f.search(&SearchRequest{TagIDs: []string{tag.ID, tag.ID}}); len(got) != 1 {
			t.Errorf("got %v, want the tagged post", postIDs(got))
		}
	})
}

// Membership is not scope. A search "in this channel" was returning hits from
// every other channel the caller belonged to.
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

	if got := f.search(&SearchRequest{ChannelID: f.channel.ID, Terms: "needle"}); len(got) != 1 || got[0].ID != here.ID {
		t.Errorf("got %v, want just the post in the requested channel", postIDs(got))
	}
}

// Scope used to be applied after the backend had cut a page, so a page of
// hits that were mostly unreadable came back nearly empty while the next page
// still held readable ones.
func TestSearchPosts_PagesAreFullAfterScoping(t *testing.T) {
	f, other := newSearchFixture(t)
	var readable []string
	for i := range 6 {
		at := int64(100 - i*10)
		readable = append(readable, f.post(f.channel, "searcher", "needle", at).ID)
		f.post(other, "outsider", "needle", at-5) // interleaved, unreadable
	}

	page0 := f.search(&SearchRequest{Terms: "needle", Page: 0, PerPage: 4})
	page1 := f.search(&SearchRequest{Terms: "needle", Page: 1, PerPage: 4})
	if got := postIDs(page0); len(got) != 4 || got[0] != readable[0] || got[3] != readable[3] {
		t.Errorf("page 0 = %v, want the 4 newest readable posts", got)
	}
	if got := postIDs(page1); len(got) != 2 || got[0] != readable[4] {
		t.Errorf("page 1 = %v, want the remaining 2", got)
	}
}

func TestSearchPosts_TeamScope(t *testing.T) {
	f, _ := newSearchFixture(t)
	in := f.post(f.channel, "searcher", "needle", 1)

	// A channel the searcher is in, on another team.
	otherTeam := &model.Channel{ID: model.NewID(), TeamID: model.NewID(), Name: "x", DisplayName: "X",
		Type: model.ChannelOpen, CreateAt: 1, UpdateAt: 1}
	f.store.Channels.Seed(otherTeam)
	f.store.Channels.SeedMember(&model.ChannelMember{ChannelID: otherTeam.ID, UserID: f.user("searcher").ID})
	f.post(otherTeam, "searcher", "needle", 2)

	if got := f.search(&SearchRequest{TeamID: f.team.ID, Terms: "needle"}); len(got) != 1 || got[0].ID != in.ID {
		t.Errorf("got %v, want only the post on the requested team", postIDs(got))
	}

	_, err := f.app.SearchPosts(t.Context(), &SearchRequest{UserID: f.user("stranger").ID, TeamID: f.team.ID, Terms: "needle", PerPage: 10})
	if !isForbidden(err) {
		t.Errorf("a non-member searched the team: %v", err)
	}
}

func TestSearchPosts_From(t *testing.T) {
	f, _ := newSearchFixture(t)
	f.join(f.channel, "alice")
	mine := f.post(f.channel, "alice", "needle", 1)
	f.post(f.channel, "searcher", "needle", 2)

	if got := f.search(&SearchRequest{Terms: "needle", From: "@Alice"}); len(got) != 1 || got[0].ID != mine.ID {
		t.Errorf("got %v, want only alice's post", postIDs(got))
	}
	if got := f.search(&SearchRequest{Terms: "needle", From: "nobody"}); len(got) != 0 {
		t.Errorf("an unknown author matched %v", postIDs(got))
	}
}

// fakeZinc answers searches with hits, from a fixed ordered list, or fails.
func fakeZinc(t *testing.T, hits []string, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		var req struct {
			From int `json:"from"`
			Max  int `json:"max_results"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		end := min(req.From+req.Max, len(hits))
		type hit struct {
			ID string `json:"_id"`
		}
		out := []hit{}
		for _, id := range hits[min(req.From, len(hits)):end] {
			out = append(out, hit{ID: id})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": map[string]any{"hits": out}})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestSearchPosts_Zinc(t *testing.T) {
	t.Run("hits are scoped and paged after filtering", func(t *testing.T) {
		f, other := newSearchFixture(t)
		var hits, readable []string
		for range 150 {
			p := f.post(other, "outsider", "needle", 1)
			hits = append(hits, p.ID)
		}
		for range 3 {
			p := f.post(f.channel, "searcher", "needle", 2)
			hits = append(hits, p.ID)
			readable = append(readable, p.ID)
		}
		f.app.Config.ZincSearchURL = fakeZinc(t, hits, http.StatusOK)

		got := postIDs(f.search(&SearchRequest{Terms: "needle", PerPage: 2}))
		if len(got) != 2 || got[0] != readable[0] || got[1] != readable[1] {
			t.Errorf("got %v, want the first 2 readable hits in Zinc order", got)
		}
		got = postIDs(f.search(&SearchRequest{Terms: "needle", Page: 1, PerPage: 2}))
		if len(got) != 1 || got[0] != readable[2] {
			t.Errorf("page 1 = %v, want the last readable hit", got)
		}
	})

	// Zinc answering with nothing is an answer. Falling back to SQL there
	// made every page past the end of Zinc's results return unrelated ILIKE
	// hits.
	t.Run("no hits is not a reason to ask SQL", func(t *testing.T) {
		f, _ := newSearchFixture(t)
		f.post(f.channel, "searcher", "needle", 1)
		f.app.Config.ZincSearchURL = fakeZinc(t, nil, http.StatusOK)

		if got := f.search(&SearchRequest{Terms: "needle"}); len(got) != 0 {
			t.Errorf("got %v from SQL after Zinc said no hits", postIDs(got))
		}
	})

	t.Run("a Zinc failure falls back to SQL", func(t *testing.T) {
		f, _ := newSearchFixture(t)
		p := f.post(f.channel, "searcher", "needle", 1)
		f.app.Config.ZincSearchURL = fakeZinc(t, nil, http.StatusInternalServerError)

		if got := f.search(&SearchRequest{Terms: "needle"}); len(got) != 1 || got[0].ID != p.ID {
			t.Errorf("got %v, want the SQL result", postIDs(got))
		}
	})
}

func isForbidden(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not a member")
}
