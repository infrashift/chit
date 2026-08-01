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

// GetTagsForPosts returns tags for many posts at once, restricted to posts in
// channels the actor belongs to. Posts they cannot read are omitted rather
// than refused, so one inaccessible ID in a batch does not fail the request.
//
// Membership is resolved once per distinct channel, not once per post.
func (a *App) GetTagsForPosts(ctx context.Context, messageIDs []string, actorID string) (map[string][]*model.Tag, error) {
	if len(messageIDs) == 0 {
		return map[string][]*model.Tag{}, nil
	}

	allowed := map[string]bool{}
	visible := make([]string, 0, len(messageIDs))

	for _, id := range messageIDs {
		post, err := a.Store.Post().Get(ctx, id)
		if err != nil {
			continue
		}
		ok, seen := allowed[post.ChannelID]
		if !seen {
			ok = a.isChannelMember(ctx, post.ChannelID, actorID)
			allowed[post.ChannelID] = ok
		}
		if ok {
			visible = append(visible, id)
		}
	}

	if len(visible) == 0 {
		return map[string][]*model.Tag{}, nil
	}
	return a.Store.Tag().GetTagsForPosts(ctx, visible)
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
