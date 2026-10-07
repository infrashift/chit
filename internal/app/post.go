package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// CreatePost creates a new post and handles threading, channel stats, mentions, and WebSocket broadcast.
func (a *App) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	// Membership comes first, before slash commands are intercepted: a command
	// runs in the context of the channel it was typed in, and `/topic` with no
	// argument reads the channel header back to whoever asked.
	if err := a.requireChannelMember(ctx, post.ChannelID, post.UserID); err != nil {
		return nil, err
	}

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

	if post.RootID != "" {
		if err := a.requireReplyableRoot(ctx, post); err != nil {
			return nil, err
		}
	}

	// Process mentions
	mentionedUserIDs, _ := a.processMentions(ctx, post)
	if len(mentionedUserIDs) > 0 {
		if post.Props == nil {
			post.Props = make(map[string]any)
		}
		post.Props["mentions"] = mentionedUserIDs
	}

	saved, err := a.Store.Post().Save(ctx, post)
	if err != nil {
		return nil, err
	}

	// Update channel stats (atomic SQL)
	if err := a.Store.Channel().IncrementMsgCount(ctx, saved.ChannelID, saved.CreateAt); err != nil {
		slog.Error("failed to increment channel message count", "channel_id", saved.ChannelID, "error", err)
	}

	// Handle threading
	if saved.RootID != "" {
		if err := a.handleThreadReply(ctx, saved); err != nil {
			// Don't fail the post creation, but the thread metadata is now stale.
			slog.Error("failed to update thread for reply", "post_id", saved.ID, "root_id", saved.RootID, "error", err)
		}
	}

	a.broadcastPostEvent(ctx, model.WebSocketEventPosted, saved)

	// Notify mentioned users after post is saved and broadcast
	if len(mentionedUserIDs) > 0 {
		a.notifyMentionedUsers(ctx, saved, mentionedUserIDs)
	}

	return saved, nil
}

// requireReplyableRoot returns a 400 unless post.RootID names a live root post
// in post's own channel. Membership was only ever checked on the reply's
// channel, so a reply could name a root in a channel the author could not
// read: the thread row was then created under the author's channel and the
// author auto-followed it, which put the private root into their thread inbox.
func (a *App) requireReplyableRoot(ctx context.Context, post *model.Post) error {
	root, err := a.Store.Post().Get(ctx, post.RootID)
	if err != nil {
		if isNotFound(err) {
			return model.NewBadRequestError("App.CreatePost", "root_id does not name a post")
		}
		return err
	}
	if root.ChannelID != post.ChannelID {
		return model.NewBadRequestError("App.CreatePost", "a reply must be in its root post's channel")
	}
	if root.RootID != "" {
		return model.NewBadRequestError("App.CreatePost", "root_id names a reply; reply to its root instead")
	}
	return nil
}

// GetPost retrieves a post by ID, requiring the caller to be a member of its channel.
func (a *App) GetPost(ctx context.Context, id, userID string) (*model.Post, error) {
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := a.requireChannelMember(ctx, post.ChannelID, userID); err != nil {
		return nil, err
	}

	return post, nil
}

// PostPatch is an edit to a post. A nil Content leaves the text unchanged.
// Props are merged key by key into the existing props, and a null value
// removes its key; props the edit does not mention are kept. The server-owned
// "mentions" prop is always recomputed from the content.
type PostPatch struct {
	Content *string
	Props   map[string]any
}

// UpdatePost edits a post on behalf of actorID, who must be the author or a
// system admin. Re-parses mentions for Props but does not re-increment
// counters. It returns, and broadcasts, the whole post: the edit used to be
// echoed back with only the fields the client sent, so post_edited carried
// create_at 0 and an empty root_id, and clients moved the post out of its
// thread.
func (a *App) UpdatePost(ctx context.Context, id string, patch PostPatch, actorID string) (*model.Post, error) {
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if authErr := a.requirePostOwner(ctx, post, actorID); authErr != nil {
		return nil, authErr
	}

	if patch.Content != nil {
		post.Content = *patch.Content
	}
	if post.Props == nil {
		post.Props = make(map[string]any)
	}
	for k, v := range patch.Props {
		if v == nil {
			delete(post.Props, k)
		} else {
			post.Props[k] = v
		}
	}
	if err := post.IsValid(); err != nil {
		return nil, err
	}

	// Re-parse mentions on plaintext for correct client rendering
	mentionedUserIDs, _ := a.processMentions(ctx, post)
	if len(mentionedUserIDs) > 0 {
		post.Props["mentions"] = mentionedUserIDs
	} else {
		delete(post.Props, "mentions")
	}

	updated, err := a.Store.Post().Update(ctx, post)
	if err != nil {
		return nil, err
	}

	a.broadcastPostEvent(ctx, model.WebSocketEventPostEdited, updated)
	return updated, nil
}

// DeletePost soft-deletes a post on behalf of actorID, who must be the author
// or a system admin.
func (a *App) DeletePost(ctx context.Context, id, actorID string) error {
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return err
	}
	if err := a.requirePostOwner(ctx, post, actorID); err != nil {
		return err
	}

	if err := a.Store.Post().Delete(ctx, id, model.GetMillis()); err != nil {
		return err
	}

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: model.WebSocketEventPostDeleted,
		Data:  map[string]any{"post_id": id, "channel_id": post.ChannelID},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	}, &pubsub.EventEnvelope{Event: model.WebSocketEventPostDeleted, PostID: id, ChannelID: post.ChannelID})

	return nil
}

// GetPostsForChannel retrieves paginated posts for a channel, requiring the
// caller to be a member.
func (a *App) GetPostsForChannel(ctx context.Context, channelID, userID string, opts model.GetPostsOptions) (*model.PostList, error) {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}

	opts.ChannelID = channelID
	list, err := a.Store.Post().GetPostsForChannel(ctx, channelID, opts)
	if err != nil {
		return nil, err
	}

	return list, nil
}

// PinPost pins a post. Pinning is a channel-level act: any channel member may pin.
func (a *App) PinPost(ctx context.Context, id, userID string) error {
	return a.setPostPinned(ctx, id, userID, true, model.WebSocketEventPostPinned)
}

// UnpinPost unpins a post. Any channel member may unpin.
func (a *App) UnpinPost(ctx context.Context, id, userID string) error {
	return a.setPostPinned(ctx, id, userID, false, model.WebSocketEventPostUnpinned)
}

func (a *App) setPostPinned(ctx context.Context, id, userID string, pinned bool, event string) error {
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return err
	}
	if err := a.requireChannelMember(ctx, post.ChannelID, userID); err != nil {
		return err
	}

	if err := a.Store.Post().SetPinned(ctx, id, pinned); err != nil {
		return err
	}

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: event,
		Data:  map[string]any{"post_id": id, "channel_id": post.ChannelID},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	}, &pubsub.EventEnvelope{Event: event, PostID: id, ChannelID: post.ChannelID})

	return nil
}

// GetPinnedPosts retrieves pinned posts for a channel, requiring the caller to
// be a member.
func (a *App) GetPinnedPosts(ctx context.Context, channelID, userID string) (*model.PostList, error) {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}

	list, err := a.Store.Post().GetPinnedPosts(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (a *App) handleThreadReply(ctx context.Context, post *model.Post) error {
	// Ensure thread record exists, then increment atomically
	thread := &model.Thread{
		PostID:       post.RootID,
		ChannelID:    post.ChannelID,
		ReplyCount:   0,
		LastReplyAt:  0,
		Participants: []string{},
	}
	if err := a.Store.Thread().SaveOrUpdate(ctx, thread); err != nil {
		return err
	}

	if err := a.Store.Thread().IncrementReplyCount(ctx, post.RootID, post.CreateAt, post.UserID); err != nil {
		return err
	}

	// Auto-follow the thread
	membership := &model.ThreadMembership{
		PostID:    post.RootID,
		UserID:    post.UserID,
		Following: true,
	}
	return a.Store.Thread().SaveMembership(ctx, membership)
}

func (a *App) broadcastPostEvent(ctx context.Context, event string, post *model.Post) {
	data, err := json.Marshal(post)
	if err != nil {
		slog.Error("failed to marshal post for broadcast", "post_id", post.ID, "error", err)
		return
	}
	var dataMap map[string]any
	if err := json.Unmarshal(data, &dataMap); err != nil {
		slog.Error("failed to build broadcast payload", "post_id", post.ID, "error", err)
		return
	}

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: event,
		Data:  dataMap,
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	}, &pubsub.EventEnvelope{Event: event, PostID: post.ID, ChannelID: post.ChannelID})
}
