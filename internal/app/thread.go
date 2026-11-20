package app

import (
	"context"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

// GetThread retrieves a thread (root post + all replies), requiring the caller
// to be a member of the thread's channel.
func (a *App) GetThread(ctx context.Context, rootID, userID string) (*model.PostList, error) {
	assert.That(a.Store != nil, "App.GetThread: nil store")
	assert.That(userID != "", "App.GetThread: empty user id")
	root, err := a.Store.Post().Get(ctx, rootID)
	if err != nil {
		return nil, err
	}
	assert.That(root != nil, "App.GetThread: nil root without an error")
	if authErr := a.requireChannelMember(ctx, root.ChannelID, userID); authErr != nil {
		return nil, authErr
	}

	list, err := a.Store.Post().GetPostsForThread(ctx, rootID)
	if err != nil {
		return nil, err
	}
	assert.That(list != nil, "App.GetThread: nil list without an error")

	return list, nil
}

// GetThreadsForUser retrieves threads the user is following within a team.
func (a *App) GetThreadsForUser(ctx context.Context, userID, teamID string, page, perPage int) (*model.UserThreadList, error) {
	assert.That(a.Store != nil, "App.GetThreadsForUser: nil store")
	assert.That(userID != "", "App.GetThreadsForUser: empty user id")
	assert.That(page >= 0 && perPage >= 1, "App.GetThreadsForUser: pagination not clamped")
	return a.Store.Thread().GetThreadsForUser(ctx, userID, teamID, page, perPage)
}

// GetDirectThreadsForUser retrieves the threads a user follows in direct and
// group channels.
func (a *App) GetDirectThreadsForUser(ctx context.Context, userID string, page, perPage int) (*model.UserThreadList, error) {
	assert.That(a.Store != nil, "App.GetDirectThreadsForUser: nil store")
	assert.That(userID != "", "App.GetDirectThreadsForUser: empty user id")
	assert.That(page >= 0 && perPage >= 1, "App.GetDirectThreadsForUser: pagination not clamped")
	return a.Store.Thread().GetDirectThreadsForUser(ctx, userID, page, perPage)
}

// requireThreadReadable returns an error unless postID is a thread root in a
// channel userID belongs to. Following a thread puts its root post in the
// follower's inbox, so without this anyone who knew a post ID could read it.
func (a *App) requireThreadReadable(ctx context.Context, postID, userID string) error {
	assert.That(a.Store != nil, "App.requireThreadReadable: nil store")
	assert.That(userID != "", "App.requireThreadReadable: empty user id")
	root, err := a.Store.Post().Get(ctx, postID)
	if err != nil {
		return err
	}
	assert.That(root != nil, "App.requireThreadReadable: nil post without an error")
	if root.RootID != "" {
		return model.NewBadRequestError("App.requireThreadReadable", "not a thread root")
	}
	return a.requireChannelMember(ctx, root.ChannelID, userID)
}

// MarkThreadAsRead marks a thread as read for a user.
func (a *App) MarkThreadAsRead(ctx context.Context, postID, userID string) error {
	assert.That(a.Store != nil, "App.MarkThreadAsRead: nil store")
	assert.That(ctx != nil, "App.MarkThreadAsRead: nil context")
	if err := a.requireThreadReadable(ctx, postID, userID); err != nil {
		return err
	}
	return a.Store.Thread().MarkAsRead(ctx, postID, userID, model.GetMillis())
}

// UpdateThreadFollowing updates a user's follow status for a thread.
func (a *App) UpdateThreadFollowing(ctx context.Context, postID, userID string, following bool) error {
	assert.That(a.Store != nil, "App.UpdateThreadFollowing: nil store")
	assert.That(ctx != nil, "App.UpdateThreadFollowing: nil context")
	if err := a.requireThreadReadable(ctx, postID, userID); err != nil {
		return err
	}
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

	assert.That(membership != nil, "App.UpdateThreadFollowing: store returned a nil membership")
	membership.Following = following
	return a.Store.Thread().UpdateMembership(ctx, membership)
}
