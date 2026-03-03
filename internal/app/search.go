package app

import (
	"context"

	"github.com/infrashift/chit/internal/jobs/workers"
	"github.com/infrashift/chit/internal/model"
)

// SearchPosts searches for posts matching a query, using ZincSearch.
func (a *App) SearchPosts(ctx context.Context, query string, page, perPage int) (*model.PostList, error) {
	indexer := workers.NewSearchIndexer(
		a.Config.ZincSearchURL,
		a.Config.ZincSearchUser,
		a.Config.ZincSearchPassword,
	)

	ids, err := indexer.Search(ctx, query, page*perPage, perPage)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return &model.PostList{Order: []*model.Post{}}, nil
	}

	var posts []*model.Post
	for _, id := range ids {
		post, err := a.Store.Post().Get(id)
		if err != nil {
			continue
		}
		_ = a.decryptPost(ctx, post)
		posts = append(posts, post)
	}

	return &model.PostList{Order: posts}, nil
}
