package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// GetThread retrieves a thread (root post + all replies).
func (a *App) GetThread(ctx context.Context, rootID string) (*model.PostList, error) {
	list, err := a.Store.Post().GetPostsForThread(rootID)
	if err != nil {
		return nil, err
	}

	for _, p := range list.Order {
		_ = a.decryptPost(ctx, p)
	}

	return list, nil
}

// GetThreadsForUser retrieves threads the user is following within a team.
func (a *App) GetThreadsForUser(userID, teamID string, page, perPage int) (*model.UserThreadList, error) {
	return a.Store.Thread().GetThreadsForUser(userID, teamID, page, perPage)
}

// MarkThreadAsRead marks a thread as read for a user.
func (a *App) MarkThreadAsRead(postID, userID string) error {
	return a.Store.Thread().MarkAsRead(postID, userID, model.GetMillis())
}

// UpdateThreadFollowing updates a user's follow status for a thread.
func (a *App) UpdateThreadFollowing(postID, userID string, following bool) error {
	membership, err := a.Store.Thread().GetMembership(postID, userID)
	if err != nil {
		// Create membership if it doesn't exist
		membership = &model.ThreadMembership{
			PostID:    postID,
			UserID:    userID,
			Following: following,
		}
		return a.Store.Thread().SaveMembership(membership)
	}

	membership.Following = following
	return a.Store.Thread().UpdateMembership(membership)
}
