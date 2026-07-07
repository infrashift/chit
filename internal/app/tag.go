package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// CreateTag creates a new tag.
func (a *App) CreateTag(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	return a.Store.Tag().Save(ctx, tag)
}

// GetAllTags retrieves all tags.
func (a *App) GetAllTags(ctx context.Context) ([]*model.Tag, error) {
	return a.Store.Tag().GetAll(ctx)
}

// AddTagToPost associates a tag with a post.
func (a *App) AddTagToPost(ctx context.Context, messageID, tagID string) error {
	return a.Store.Tag().AddTagToPost(ctx, messageID, tagID)
}

// RemoveTagFromPost removes a tag association from a post.
func (a *App) RemoveTagFromPost(ctx context.Context, messageID, tagID string) error {
	return a.Store.Tag().RemoveTagFromPost(ctx, messageID, tagID)
}

// GetTagsForPost retrieves tags associated with a post.
func (a *App) GetTagsForPost(ctx context.Context, messageID string) ([]*model.Tag, error) {
	return a.Store.Tag().GetTagsForPost(ctx, messageID)
}
