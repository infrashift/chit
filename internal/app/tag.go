package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// Tag operations are authorized against the channel the post belongs to. A tag
// is metadata about a post, so being able to read or change it is the same
// privilege as reading the post itself; without this check any authenticated
// user could tag, untag, or enumerate tags on any post ID.

// CreateTag creates a new tag. Tags are a workspace-wide vocabulary rather
// than per-channel, so any authenticated user may add one.
func (a *App) CreateTag(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	return a.Store.Tag().Save(ctx, tag)
}

// GetAllTags retrieves all tags.
func (a *App) GetAllTags(ctx context.Context) ([]*model.Tag, error) {
	return a.Store.Tag().GetAll(ctx)
}

// AddTagToPost associates a tag with a post, requiring the actor to be a
// member of the post's channel.
func (a *App) AddTagToPost(ctx context.Context, messageID, tagID, actorID string) error {
	if err := a.requirePostChannelMember(ctx, messageID, actorID); err != nil {
		return err
	}
	return a.Store.Tag().AddTagToPost(ctx, messageID, tagID)
}

// RemoveTagFromPost removes a tag association, requiring the actor to be a
// member of the post's channel.
func (a *App) RemoveTagFromPost(ctx context.Context, messageID, tagID, actorID string) error {
	if err := a.requirePostChannelMember(ctx, messageID, actorID); err != nil {
		return err
	}
	return a.Store.Tag().RemoveTagFromPost(ctx, messageID, tagID)
}

// GetTagsForPost retrieves tags for a post, requiring the actor to be a member
// of the post's channel.
func (a *App) GetTagsForPost(ctx context.Context, messageID, actorID string) ([]*model.Tag, error) {
	if err := a.requirePostChannelMember(ctx, messageID, actorID); err != nil {
		return nil, err
	}
	return a.Store.Tag().GetTagsForPost(ctx, messageID)
}

// requirePostChannelMember resolves a post to its channel and checks
// membership there.
func (a *App) requirePostChannelMember(ctx context.Context, messageID, actorID string) error {
	post, err := a.Store.Post().Get(ctx, messageID)
	if err != nil {
		return err
	}
	return a.requireChannelMember(ctx, post.ChannelID, actorID)
}
