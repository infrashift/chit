package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/infrashift/chit/internal/model"
)

// Zinc scanning bounds. The index cannot filter by channel or author, so a
// search through it reads hits in batches and filters them here; this caps
// how far it reads before settling for what it has found.
const (
	zincScanBatch = 100
	zincScanLimit = 1000
)

// SearchRequest is a post search on UserID's behalf. At most one of TeamID and
// ChannelID narrows the scope; with neither, the search spans every channel
// the user belongs to. From limits results to one author by username (a
// leading @ is accepted). Page is zero-based.
type SearchRequest struct {
	UserID    string
	TeamID    string
	ChannelID string
	Terms     string
	TagIDs    []string
	From      string
	Page      int
	PerPage   int
}

// SearchPosts runs a search and returns one page of results.
//
// Scope (the channels the user may read, narrowed by team or channel) and the
// author filter are resolved first and applied BEFORE pagination. They used
// to be applied to a page the backend had already cut, so pages came back
// short or empty while later pages still held hits, and a team search was
// not scoped to the team at all.
func (a *App) SearchPosts(ctx context.Context, req *SearchRequest) (*model.PostList, error) {
	empty := &model.PostList{Order: []*model.Post{}}

	// Only Tags the searcher can see may filter: a private Tag of somebody
	// else, like an unknown Tag, matches nothing, so the result never tells
	// whether it exists or what it is on.
	tagIDs, visible, err := a.visibleTagIDs(ctx, req.TagIDs, req.UserID, "App.SearchPosts")
	if err != nil {
		return nil, err
	}
	if !visible {
		return empty, nil
	}
	if req.Terms == "" && len(tagIDs) == 0 {
		return empty, nil
	}

	channelIDs, err := a.searchScope(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(channelIDs) == 0 {
		return empty, nil
	}

	q := &model.PostSearch{
		Terms: req.Terms, TagIDs: tagIDs, ChannelIDs: channelIDs,
		Page: req.Page, PerPage: req.PerPage,
	}
	if req.From != "" {
		authorID, found, lookupErr := a.searchAuthorID(ctx, req.From)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !found {
			// An unknown author matches nothing, which is more useful than
			// silently returning everything.
			return empty, nil
		}
		q.AuthorID = authorID
	}

	posts, err := a.runPostSearch(ctx, q)
	if err != nil {
		return nil, err
	}
	if posts == nil {
		posts = []*model.Post{}
	}
	return &model.PostList{Order: posts}, nil
}

// searchAuthorID resolves a search's author filter, a username with an
// optional leading @, to a user ID. found is false for an unknown username.
func (a *App) searchAuthorID(ctx context.Context, from string) (id string, found bool, err error) {
	author, err := a.Store.User().GetByUsername(ctx, strings.ToLower(strings.TrimPrefix(from, "@")))
	if err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return author.ID, true, nil
}

// runPostSearch runs q through ZincSearch when it has terms and Zinc is
// configured, and through SQL otherwise.
func (a *App) runPostSearch(ctx context.Context, q *model.PostSearch) ([]*model.Post, error) {
	if q.Terms == "" || a.Config.ZincSearchURL == "" {
		return a.Store.Post().Search(ctx, q)
	}
	posts, err := a.searchZinc(ctx, q)
	if err != nil {
		// Zinc being down is not a reason to fail the search. Zinc
		// answering with no hits is an answer, and is not second-guessed
		// with a full-table ILIKE.
		slog.Warn("search: ZincSearch failed, falling back to SQL", "error", err)
		return a.Store.Post().Search(ctx, q)
	}
	return posts, nil
}

// searchScope returns the channels req may search, after checking the caller
// may search them.
func (a *App) searchScope(ctx context.Context, req *SearchRequest) ([]string, error) {
	switch {
	case req.ChannelID != "":
		if err := a.requireChannelMember(ctx, req.ChannelID, req.UserID); err != nil {
			return nil, err
		}
		return []string{req.ChannelID}, nil
	case req.TeamID != "":
		if err := a.requireTeamMember(ctx, req.TeamID, req.UserID); err != nil {
			return nil, err
		}
		channels, err := a.Store.Channel().GetChannelsForUser(ctx, req.UserID, req.TeamID)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(channels))
		for _, c := range channels {
			ids = append(ids, c.ID)
		}
		return ids, nil
	default:
		return a.Store.Channel().GetChannelIDsForUser(ctx, req.UserID)
	}
}

// searchZinc pages through ZincSearch hits, keeping those that fall in q's
// scope, until it has q's page or has read zincScanLimit hits.
func (a *App) searchZinc(ctx context.Context, q *model.PostSearch) ([]*model.Post, error) {
	inScope := make(map[string]bool, len(q.ChannelIDs))
	for _, id := range q.ChannelIDs {
		inScope[id] = true
	}
	skip := q.Page * q.PerPage
	out := make([]*model.Post, 0, q.PerPage)

	for from := 0; from < zincScanLimit; from += zincScanBatch {
		ids, err := a.searchClient().Search(ctx, q.Terms, from, zincScanBatch)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}

		matches, err := a.zincBatchMatches(ctx, q, ids, inScope)
		if err != nil {
			return nil, err
		}
		for _, p := range matches {
			if skip > 0 {
				skip--
				continue
			}
			out = append(out, p)
			if len(out) == q.PerPage {
				return out, nil
			}
		}
		if len(ids) < zincScanBatch {
			break
		}
	}
	return out, nil
}

// zincBatchMatches loads one batch of Zinc hits and returns, in hit order
// (Zinc's relevance order), the posts that fall in q's scope: an allowed
// channel, q's author if set, and every one of q's tags if set.
func (a *App) zincBatchMatches(ctx context.Context, q *model.PostSearch, ids []string, inScope map[string]bool) ([]*model.Post, error) {
	posts, err := a.Store.Post().GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Post, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	tagged := map[string]bool{}
	if len(q.TagIDs) > 0 {
		matches, err := a.Store.Tag().FilterPostIDsByTags(ctx, ids, q.TagIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range matches {
			tagged[id] = true
		}
	}

	out := make([]*model.Post, 0, len(ids))
	for _, id := range ids {
		p := byID[id]
		if p == nil || !inScope[p.ChannelID] ||
			(q.AuthorID != "" && p.UserID != q.AuthorID) ||
			(len(q.TagIDs) > 0 && !tagged[id]) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
