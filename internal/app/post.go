package app

import (
	"context"
	"encoding/json"

	"github.com/infrashift/chit/internal/model"
)

// CreatePost creates a new post and handles threading, channel stats, mentions, and WebSocket broadcast.
func (a *App) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	// Intercept slash commands — they are NOT persisted.
	if a.CommandRegistry != nil {
		resp, handled, err := a.InterceptSlashCommand(ctx, post.UserID, post.ChannelID, post.Content)
		if err != nil {
			return nil, err
		}
		if handled {
			return &model.Post{
				ID:        model.NewID(),
				ChannelID: post.ChannelID,
				UserID:    post.UserID,
				Content:   resp.Text,
				Type:      "command_response",
				CreateAt:  model.GetMillis(),
				Props:     map[string]any{"ephemeral": true},
			}, nil
		}
	}

	// Process mentions on plaintext content before encryption
	mentionedUserIDs, _ := a.processMentions(post)
	if len(mentionedUserIDs) > 0 {
		if post.Props == nil {
			post.Props = make(map[string]any)
		}
		post.Props["mentions"] = mentionedUserIDs
	}

	// Optionally encrypt content
	if a.Config.VaultEnabled {
		ciphertext, err := a.EncryptContent(ctx, post.Content)
		if err != nil {
			return nil, err
		}
		post.ContentEncrypted = ciphertext
		post.Content = ""
	}

	saved, err := a.Store.Post().Save(post)
	if err != nil {
		return nil, err
	}

	// Update channel stats (atomic SQL)
	_ = a.Store.Channel().IncrementMsgCount(saved.ChannelID, saved.CreateAt)

	// Handle threading
	if saved.RootID != "" {
		if err := a.handleThreadReply(saved); err != nil {
			// Log but don't fail the post creation
		}
	}

	a.broadcastPostEvent(model.WebSocketEventPosted, saved)

	// Notify mentioned users after post is saved and broadcast
	if len(mentionedUserIDs) > 0 {
		a.notifyMentionedUsers(saved, mentionedUserIDs)
	}

	return saved, nil
}

// GetPost retrieves a post by ID, decrypting content if needed.
func (a *App) GetPost(ctx context.Context, id string) (*model.Post, error) {
	post, err := a.Store.Post().Get(id)
	if err != nil {
		return nil, err
	}

	if err := a.decryptPost(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

// UpdatePost updates a post's content. Re-parses mentions for Props but does not re-increment counters.
func (a *App) UpdatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	// Fetch original post to carry forward immutable fields (ChannelID, UserID)
	// needed for mention resolution when the update payload omits them.
	existing, err := a.Store.Post().Get(post.ID)
	if err != nil {
		return nil, err
	}
	if post.ChannelID == "" {
		post.ChannelID = existing.ChannelID
	}
	if post.UserID == "" {
		post.UserID = existing.UserID
	}

	// Re-parse mentions on plaintext for correct client rendering
	mentionedUserIDs, _ := a.processMentions(post)
	if post.Props == nil {
		post.Props = make(map[string]any)
	}
	if len(mentionedUserIDs) > 0 {
		post.Props["mentions"] = mentionedUserIDs
	} else {
		delete(post.Props, "mentions")
	}

	if a.Config.VaultEnabled {
		ciphertext, err := a.EncryptContent(ctx, post.Content)
		if err != nil {
			return nil, err
		}
		post.ContentEncrypted = ciphertext
		post.Content = ""
	}

	updated, err := a.Store.Post().Update(post)
	if err != nil {
		return nil, err
	}

	a.broadcastPostEvent(model.WebSocketEventPostEdited, updated)
	return updated, nil
}

// DeletePost soft-deletes a post.
func (a *App) DeletePost(id string) error {
	post, err := a.Store.Post().Get(id)
	if err != nil {
		return err
	}

	if err := a.Store.Post().Delete(id, model.GetMillis()); err != nil {
		return err
	}

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventPostDeleted,
		Data:  map[string]any{"post_id": id, "channel_id": post.ChannelID},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	})

	return nil
}

// GetPostsForChannel retrieves paginated posts for a channel.
func (a *App) GetPostsForChannel(ctx context.Context, channelID string, opts model.GetPostsOptions) (*model.PostList, error) {
	opts.ChannelID = channelID
	list, err := a.Store.Post().GetPostsForChannel(channelID, opts)
	if err != nil {
		return nil, err
	}

	for _, p := range list.Order {
		_ = a.decryptPost(ctx, p)
	}

	return list, nil
}

// PinPost pins a post.
func (a *App) PinPost(id string) error {
	if err := a.Store.Post().SetPinned(id, true); err != nil {
		return err
	}

	post, _ := a.Store.Post().Get(id)
	if post != nil {
		a.Hub.Broadcast(&model.WebSocketEvent{
			Event: model.WebSocketEventPostPinned,
			Data:  map[string]any{"post_id": id, "channel_id": post.ChannelID},
			Broadcast: &model.WebSocketBroadcast{
				ChannelID: post.ChannelID,
			},
		})
	}

	return nil
}

// UnpinPost unpins a post.
func (a *App) UnpinPost(id string) error {
	if err := a.Store.Post().SetPinned(id, false); err != nil {
		return err
	}

	post, _ := a.Store.Post().Get(id)
	if post != nil {
		a.Hub.Broadcast(&model.WebSocketEvent{
			Event: model.WebSocketEventPostUnpinned,
			Data:  map[string]any{"post_id": id, "channel_id": post.ChannelID},
			Broadcast: &model.WebSocketBroadcast{
				ChannelID: post.ChannelID,
			},
		})
	}

	return nil
}

// GetPinnedPosts retrieves pinned posts for a channel.
func (a *App) GetPinnedPosts(ctx context.Context, channelID string) (*model.PostList, error) {
	list, err := a.Store.Post().GetPinnedPosts(channelID)
	if err != nil {
		return nil, err
	}
	for _, p := range list.Order {
		_ = a.decryptPost(ctx, p)
	}
	return list, nil
}

func (a *App) handleThreadReply(post *model.Post) error {
	// Ensure thread record exists, then increment atomically
	thread := &model.Thread{
		PostID:       post.RootID,
		ChannelID:    post.ChannelID,
		ReplyCount:   0,
		LastReplyAt:  0,
		Participants: []string{},
	}
	_ = a.Store.Thread().SaveOrUpdate(thread)

	if err := a.Store.Thread().IncrementReplyCount(post.RootID, post.CreateAt, post.UserID); err != nil {
		return err
	}

	// Auto-follow the thread
	membership := &model.ThreadMembership{
		PostID:    post.RootID,
		UserID:    post.UserID,
		Following: true,
	}
	return a.Store.Thread().SaveMembership(membership)
}

func (a *App) decryptPost(ctx context.Context, post *model.Post) error {
	if a.Config.VaultEnabled && len(post.ContentEncrypted) > 0 {
		plaintext, err := a.DecryptContent(ctx, post.ContentEncrypted)
		if err != nil {
			return err
		}
		post.Content = plaintext
	}
	return nil
}

func (a *App) broadcastPostEvent(event string, post *model.Post) {
	data, _ := json.Marshal(post)
	var dataMap map[string]any
	json.Unmarshal(data, &dataMap)

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: event,
		Data:  dataMap,
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	})
}
