package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// GetThread retrieves a thread (root post + all replies), requiring the caller
// to be a member of the thread's channel.
func (a *App) GetThread(ctx context.Context, rootID, userID string) (*model.PostList, error) {
	root, err := a.Store.Post().Get(ctx, rootID)
	if err != nil {
		return nil, err
	}
	if authErr := a.requireChannelMember(ctx, root.ChannelID, userID); authErr != nil {
		return nil, authErr
	}

	list, err := a.Store.Post().GetPostsForThread(ctx, rootID)
	if err != nil {
		return nil, err
	}

	return list, nil
}

// GetThreadsForUser retrieves threads the user is following within a team.
func (a *App) GetThreadsForUser(ctx context.Context, userID, teamID string, page, perPage int) (*model.UserThreadList, error) {
	return a.Store.Thread().GetThreadsForUser(ctx, userID, teamID, page, perPage)
}

// MarkThreadAsRead marks a thread as read for a user.
func (a *App) MarkThreadAsRead(ctx context.Context, postID, userID string) error {
	return a.Store.Thread().MarkAsRead(ctx, postID, userID, model.GetMillis())
}

// UpdateThreadFollowing updates a user's follow status for a thread.
func (a *App) UpdateThreadFollowing(ctx context.Context, postID, userID string, following bool) error {
	membership, err := a.Store.Thread().GetMembership(ctx, postID, userID)
	if err != nil {
		// Create membership if it doesn't exist
		membership = &model.ThreadMembership{
			PostID:    postID,
			UserID:    userID,
			Following: following,
		}
		return a.Store.Thread().SaveMembership(ctx, membership)
	}

	membership.Following = following
	return a.Store.Thread().UpdateMembership(ctx, membership)
}
