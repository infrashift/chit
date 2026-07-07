package app

import (
	"context"
	"log"

	"github.com/infrashift/chit/internal/model"
)

// searchByContent performs a SQL ILIKE fallback search.
func (a *App) searchByContent(ctx context.Context, channelID, query string, page, perPage int) (*model.PostList, error) {
	posts, err := a.Store.Post().SearchByContent(ctx, channelID, query, page, perPage)
	if err != nil {
		return nil, err
	}
	return &model.PostList{Order: posts}, nil
}

// zincSearch tries ZincSearch; on failure it falls back to SQL ILIKE.
func (a *App) zincSearch(ctx context.Context, channelID, query string, page, perPage int) ([]string, bool, error) {
	if a.Config.ZincSearchURL == "" {
		return nil, false, nil
	}
	ids, err := a.searchClient().Search(ctx, query, page*perPage, perPage)
	if err != nil {
		log.Printf("ZincSearch unavailable, falling back to SQL: %v", err)
		return nil, false, nil
	}
	return ids, true, nil
}

// SearchPosts searches for posts matching a query and/or tags on behalf of
// userID. When channelID is non-empty, results are scoped to that channel and
// the caller must be a member; otherwise results are filtered to channels the
// caller is a member of.
func (a *App) SearchPosts(ctx context.Context, channelID, userID, query string, tagIDs []string, page, perPage int) (*model.PostList, error) {
	if channelID != "" {
		if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
			return nil, err
		}
	}

	hasQuery := query != ""
	hasTags := len(tagIDs) > 0

	var ids []string

	switch {
	case hasTags && !hasQuery:
		// Tag-only search (already DB-based)
		var err error
		ids, err = a.Store.Tag().GetPostIDsByTags(ctx, tagIDs, page, perPage)
		if err != nil {
			return nil, err
		}

	case hasQuery && !hasTags:
		zincIDs, ok, err := a.zincSearch(ctx, channelID, query, page, perPage)
		if err != nil {
			return nil, err
		}
		if !ok || len(zincIDs) == 0 {
			return a.searchByContent(ctx, channelID, query, page, perPage)
		}
		ids = zincIDs

	case hasQuery && hasTags:
		zincIDs, ok, err := a.zincSearch(ctx, channelID, query, page, perPage*3)
		if err != nil {
			return nil, err
		}
		if !ok || len(zincIDs) == 0 {
			// SQL fallback: search by content, then filter by tags
			posts, err := a.Store.Post().SearchByContent(ctx, channelID, query, page, perPage*3)
			if err != nil {
				return nil, err
			}
			if len(posts) > 0 {
				candidateIDs := make([]string, len(posts))
				for i, p := range posts {
					candidateIDs[i] = p.ID
				}
				ids, err = a.Store.Tag().FilterPostIDsByTags(ctx, candidateIDs, tagIDs)
				if err != nil {
					return nil, err
				}
			}
		} else {
			ids, err = a.Store.Tag().FilterPostIDsByTags(ctx, zincIDs, tagIDs)
			if err != nil {
				return nil, err
			}
		}

	default:
		return &model.PostList{Order: []*model.Post{}}, nil
	}

	if len(ids) == 0 {
		return &model.PostList{Order: []*model.Post{}}, nil
	}

	// Filter results to channels the caller can access. Membership is checked
	// once per distinct channel in the result set.
	allowed := map[string]bool{}
	if channelID != "" {
		allowed[channelID] = true // verified above
	}

	var posts []*model.Post
	for _, id := range ids {
		post, err := a.Store.Post().Get(ctx, id)
		if err != nil {
			continue
		}
		ok, seen := allowed[post.ChannelID]
		if !seen {
			ok = a.isChannelMember(ctx, post.ChannelID, userID)
			allowed[post.ChannelID] = ok
		}
		if !ok {
			continue
		}
		posts = append(posts, post)
	}

	return &model.PostList{Order: posts}, nil
}
