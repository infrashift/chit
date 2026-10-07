package app

import (
	"context"
	"log/slog"
	"slices"
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

	tagIDs := slices.Compact(slices.Sorted(slices.Values(req.TagIDs)))
	for _, id := range tagIDs {
		if !model.IsValidID(id) {
			return nil, model.NewBadRequestError("App.SearchPosts", "invalid tag id")
		}
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
		author, lookupErr := a.Store.User().GetByUsername(ctx, strings.ToLower(strings.TrimPrefix(req.From, "@")))
		if lookupErr != nil {
			if isNotFound(lookupErr) {
				// An unknown author matches nothing, which is more useful than
				// silently returning everything.
				return empty, nil
			}
			return nil, lookupErr
		}
		q.AuthorID = author.ID
	}

	var posts []*model.Post
	if req.Terms != "" && a.Config.ZincSearchURL != "" {
		posts, err = a.searchZinc(ctx, q)
		if err != nil {
			// Zinc being down is not a reason to fail the search. Zinc
			// answering with no hits is an answer, and is not second-guessed
			// with a full-table ILIKE.
			slog.Warn("search: ZincSearch failed, falling back to SQL", "error", err)
			posts, err = a.Store.Post().Search(ctx, q)
		}
	} else {
		posts, err = a.Store.Post().Search(ctx, q)
	}
	if err != nil {
		return nil, err
	}
	if posts == nil {
		posts = []*model.Post{}
	}
	return &model.PostList{Order: posts}, nil
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

		// Walk in hit order, which is Zinc's relevance order.
		for _, id := range ids {
			p := byID[id]
			if p == nil || !inScope[p.ChannelID] ||
				(q.AuthorID != "" && p.UserID != q.AuthorID) ||
				(len(q.TagIDs) > 0 && !tagged[id]) {
				continue
			}
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
