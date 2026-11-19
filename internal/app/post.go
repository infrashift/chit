package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// CreatePost creates a new post and handles threading, channel stats, mentions, and WebSocket broadcast.
func (a *App) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	assert.That(post != nil, "App.CreatePost: nil post")
	assert.That(ctx != nil, "App.CreatePost: nil context")
	return a.CreatePostAs(ctx, post, nil)
}

// CreatePostAs creates post as CreatePost does. A non-nil actor is the
// allowlisted machine actor posting on behalf of post.UserID (see
// ResolvePrincipal): the post is authorized as post.UserID, and the actor is
// recorded in props.on_behalf_of and in the audit log.
func (a *App) CreatePostAs(ctx context.Context, post *model.Post, actor *model.User) (*model.Post, error) {
	assert.That(post != nil, "App.CreatePostAs: nil post")
	assert.That(a.Store != nil, "App.CreatePostAs: nil store")
	if err := a.authorizePostCreate(ctx, post, actor); err != nil {
		return nil, err
	}

	// Intercept slash commands — they are NOT persisted.
	if a.CommandRegistry != nil {
		resp, handled, err := a.InterceptSlashCommand(ctx, post.UserID, post.ChannelID, post.Content)
		if err != nil {
			return nil, err
		}
		if handled {
			assert.That(resp != nil, "App.CreatePostAs: handled command has no response")
			return commandResponsePost(post, resp), nil
		}
	}

	var root *model.Post
	if post.RootID != "" {
		var err error
		if root, err = a.requireReplyableRoot(ctx, post); err != nil {
			return nil, err
		}
	}

	// Process mentions. A failed lookup must not fail the post: it is
	// logged, and the post keeps the IDs processMentions returned.
	mentionedUserIDs, err := a.processMentions(ctx, post)
	errlog.Warn(err, "process mentions")
	if len(mentionedUserIDs) > 0 {
		setPostProp(post, "mentions", mentionedUserIDs)
	}

	if actor != nil {
		setPostProp(post, model.PropOnBehalfOf, map[string]any{"actor_id": actor.ID})
	}

	saved, err := a.Store.Post().Save(ctx, post)
	if err != nil {
		return nil, err
	}
	assert.That(saved != nil && saved.ID != "", "App.CreatePostAs: saved post has no id")
	if actor != nil && a.AuditLogger != nil {
		a.AuditLogger.LogPostOnBehalfOf(model.NewID(), actor.ID, actor.OAuthClientID, saved.UserID, saved.ID, saved.ChannelID)
	}

	a.afterPostSaved(ctx, saved, root, mentionedUserIDs)
	return saved, nil
}

// authorizePostCreate runs CreatePostAs's checks that come before anything
// else: channel membership, the on-behalf-of allowlist, and provenance.
func (a *App) authorizePostCreate(ctx context.Context, post *model.Post, actor *model.User) error {
	assert.That(post != nil, "App.authorizePostCreate: nil post")
	assert.That(post.UserID != "", "App.authorizePostCreate: post has no author")
	// Membership comes first, before slash commands are intercepted: a command
	// runs in the context of the channel it was typed in, and `/topic` with no
	// argument reads the channel header back to whoever asked.
	if err := a.requireChannelMember(ctx, post.ChannelID, post.UserID); err != nil {
		return err
	}

	// Server-owned: only the server records who acted for whom.
	delete(post.Props, model.PropOnBehalfOf)
	_, kept := post.Props[model.PropOnBehalfOf]
	assert.That(!kept, "App.authorizePostCreate: client-supplied on_behalf_of survived")
	if actor != nil && !a.OnBehalfOfAllowed(actor) {
		return model.NewForbiddenError("App.CreatePost", "this client may not act on behalf of a user")
	}
	if prov, ok := post.Props[model.PropProvenance]; ok {
		if err := a.requireProvenanceWriter(ctx, post.UserID, actor); err != nil {
			return err
		}
		if err := model.ValidateProvenance(prov); err != nil {
			return err
		}
	}
	return nil
}

// commandResponsePost is the ephemeral, unsaved post that answers a slash
// command typed as post.
func commandResponsePost(post *model.Post, resp *CommandResponse) *model.Post {
	assert.That(post != nil, "commandResponsePost: nil post")
	assert.That(resp != nil, "commandResponsePost: nil response")
	return &model.Post{
		ID:        model.NewID(),
		ChannelID: post.ChannelID,
		UserID:    post.UserID,
		Content:   resp.Text,
		Type:      "command_response",
		CreateAt:  model.GetMillis(),
		Props:     map[string]any{"ephemeral": true},
	}
}

// setPostProp sets one prop, creating the props map when the post has none.
func setPostProp(post *model.Post, key string, value any) {
	assert.That(post != nil, "setPostProp: nil post")
	assert.That(key != "", "setPostProp: empty key")
	if post.Props == nil {
		post.Props = make(map[string]any)
	}
	post.Props[key] = value
}

// afterPostSaved does the work that follows a saved post: channel stats,
// thread metadata, the WebSocket broadcast, and mention notifications. None
// of it fails the post.
func (a *App) afterPostSaved(ctx context.Context, saved, root *model.Post, mentionedUserIDs []string) {
	assert.That(saved != nil && saved.ID != "", "App.afterPostSaved: post is not saved")
	assert.That((saved.RootID == "") == (root == nil), "App.afterPostSaved: reply and root disagree")
	assert.That(a.Store != nil, "App.afterPostSaved: nil store")
	// Update channel stats (atomic SQL)
	if err := a.Store.Channel().IncrementMsgCount(ctx, saved.ChannelID, saved.CreateAt); err != nil {
		slog.Error("failed to increment channel message count", "channel_id", saved.ChannelID, "error", err)
	}

	// Handle threading
	if saved.RootID != "" {
		if err := a.handleThreadReply(ctx, saved, root); err != nil {
			// Don't fail the post creation, but the thread metadata is now stale.
			slog.Error("failed to update thread for reply", "post_id", saved.ID, "root_id", saved.RootID, "error", err)
		}
	}

	a.broadcastPostEvent(ctx, model.WebSocketEventPosted, saved)

	// Notify mentioned users after post is saved and broadcast
	if len(mentionedUserIDs) > 0 {
		a.notifyMentionedUsers(ctx, saved, mentionedUserIDs)
	}
}

// requireReplyableRoot returns a 400 unless post.RootID names a live root post
// in post's own channel. Membership was only ever checked on the reply's
// channel, so a reply could name a root in a channel the author could not
// read: the thread row was then created under the author's channel and the
// author auto-followed it, which put the private root into their thread inbox.
func (a *App) requireReplyableRoot(ctx context.Context, post *model.Post) (*model.Post, error) {
	assert.That(post != nil && post.RootID != "", "App.requireReplyableRoot: post is not a reply")
	assert.That(a.Store != nil, "App.requireReplyableRoot: nil store")
	root, err := a.Store.Post().Get(ctx, post.RootID)
	if err != nil {
		if isNotFound(err) {
			return nil, model.NewBadRequestError("App.CreatePost", "root_id does not name a post")
		}
		return nil, err
	}
	assert.That(root != nil, "App.requireReplyableRoot: nil root without an error")
	if root.ChannelID != post.ChannelID {
		return nil, model.NewBadRequestError("App.CreatePost", "a reply must be in its root post's channel")
	}
	if root.RootID != "" {
		return nil, model.NewBadRequestError("App.CreatePost", "root_id names a reply; reply to its root instead")
	}
	return root, nil
}

// requireProvenancePatch checks an edit's provenance prop, if it has one:
// setting or removing provenance both need the right to write it, and a value
// that is set must be valid.
func (a *App) requireProvenancePatch(ctx context.Context, actorID string, props map[string]any) error {
	assert.That(actorID != "", "App.requireProvenancePatch: empty actor id")
	_, hasOBO := props[model.PropOnBehalfOf]
	assert.That(!hasOBO, "App.requireProvenancePatch: on_behalf_of not removed from the patch")
	prov, ok := props[model.PropProvenance]
	if !ok {
		return nil
	}
	if err := a.requireProvenanceWriter(ctx, actorID, nil); err != nil {
		return err
	}
	if prov != nil {
		if err := model.ValidateProvenance(prov); err != nil {
			return err
		}
	}
	return nil
}

// GetPost retrieves a post by ID, requiring the caller to be a member of its channel.
func (a *App) GetPost(ctx context.Context, id, userID string) (*model.Post, error) {
	assert.That(a.Store != nil, "App.GetPost: nil store")
	assert.That(userID != "", "App.GetPost: empty user id")
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	assert.That(post != nil, "App.GetPost: nil post without an error")

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
	assert.That(a.Store != nil, "App.UpdatePost: nil store")
	assert.That(actorID != "", "App.UpdatePost: empty actor id")
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	assert.That(post != nil, "App.UpdatePost: nil post without an error")
	if authErr := a.requirePostOwner(ctx, post, actorID); authErr != nil {
		return nil, authErr
	}

	// on_behalf_of is the server's record of who posted: no edit changes it.
	delete(patch.Props, model.PropOnBehalfOf)
	if provErr := a.requireProvenancePatch(ctx, actorID, patch.Props); provErr != nil {
		return nil, provErr
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
	assert.That(post.Props != nil, "App.UpdatePost: nil props after merge")
	if err := post.IsValid(); err != nil {
		return nil, err
	}

	// Re-parse mentions on plaintext for correct client rendering. A failed
	// lookup must not fail the edit: it is logged, and the post keeps the IDs
	// processMentions returned.
	mentionedUserIDs, err := a.processMentions(ctx, post)
	errlog.Warn(err, "process mentions")
	if len(mentionedUserIDs) > 0 {
		post.Props["mentions"] = mentionedUserIDs
	} else {
		delete(post.Props, "mentions")
	}

	updated, err := a.Store.Post().Update(ctx, post)
	if err != nil {
		return nil, err
	}

	assert.That(updated != nil && updated.ID == post.ID, "App.UpdatePost: store returned another post")
	a.broadcastPostEvent(ctx, model.WebSocketEventPostEdited, updated)
	return updated, nil
}

// DeletePost soft-deletes a post on behalf of actorID, who must be the author
// or a system admin.
func (a *App) DeletePost(ctx context.Context, id, actorID string) error {
	assert.That(a.Store != nil, "App.DeletePost: nil store")
	assert.That(actorID != "", "App.DeletePost: empty actor id")
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return err
	}
	assert.That(post != nil, "App.DeletePost: nil post without an error")
	if err := a.requirePostOwner(ctx, post, actorID); err != nil {
		return err
	}

	if err := a.Store.Post().Delete(ctx, id, model.GetMillis()); err != nil {
		return err
	}
	if post.RootID != "" {
		if err := a.Store.Thread().DecrementReplyCount(ctx, post.RootID); err != nil {
			slog.Error("failed to update thread after reply deletion", "post_id", id, "root_id", post.RootID, "error", err)
		} else {
			a.broadcastThreadUpdated(ctx, post.RootID)
		}
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
	assert.That(a.Store != nil, "App.GetPostsForChannel: nil store")
	assert.That(userID != "", "App.GetPostsForChannel: empty user id")
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}

	list, err := a.Store.Post().GetPostsForChannel(ctx, channelID, opts)
	if err != nil {
		return nil, err
	}
	assert.That(list != nil, "App.GetPostsForChannel: nil list without an error")

	return list, nil
}

// PinPost pins a post. Pinning is a channel-level act: any channel member may pin.
func (a *App) PinPost(ctx context.Context, id, userID string) error {
	assert.That(userID != "", "App.PinPost: empty user id")
	return a.setPostPinned(ctx, id, userID, true, model.WebSocketEventPostPinned)
}

// UnpinPost unpins a post. Any channel member may unpin.
func (a *App) UnpinPost(ctx context.Context, id, userID string) error {
	assert.That(userID != "", "App.UnpinPost: empty user id")
	return a.setPostPinned(ctx, id, userID, false, model.WebSocketEventPostUnpinned)
}

func (a *App) setPostPinned(ctx context.Context, id, userID string, pinned bool, event string) error {
	assert.That(a.Store != nil, "App.setPostPinned: nil store")
	assert.That(event == model.WebSocketEventPostPinned || event == model.WebSocketEventPostUnpinned,
		"App.setPostPinned: event is not a pin event")
	assert.That(pinned == (event == model.WebSocketEventPostPinned), "App.setPostPinned: pinned and event disagree")
	post, err := a.Store.Post().Get(ctx, id)
	if err != nil {
		return err
	}
	assert.That(post != nil, "App.setPostPinned: nil post without an error")
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
	assert.That(a.Store != nil, "App.GetPinnedPosts: nil store")
	assert.That(userID != "", "App.GetPinnedPosts: empty user id")
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}

	list, err := a.Store.Post().GetPinnedPosts(ctx, channelID)
	if err != nil {
		return nil, err
	}
	assert.That(list != nil, "App.GetPinnedPosts: nil list without an error")
	return list, nil
}

// handleThreadReply updates root's thread for the reply post: it creates the
// thread row if needed, advances its counters, and keeps the participants'
// memberships current.
func (a *App) handleThreadReply(ctx context.Context, post, root *model.Post) error {
	assert.That(post != nil && root != nil, "App.handleThreadReply: nil reply or root")
	assert.That(post.RootID == root.ID, "App.handleThreadReply: reply is not in this thread")
	assert.That(root.RootID == "", "App.handleThreadReply: root is itself a reply")
	assert.That(post.CreateAt > 0, "App.handleThreadReply: reply is not saved")
	thread := &model.Thread{
		PostID:       root.ID,
		ChannelID:    root.ChannelID,
		Participants: []string{},
	}
	if err := a.Store.Thread().SaveOrUpdate(ctx, thread); err != nil {
		return err
	}
	if err := a.Store.Thread().IncrementReplyCount(ctx, root.ID, post.CreateAt, post.UserID); err != nil {
		return err
	}

	// Replying follows the thread, and the replier has read it up to their
	// own reply. The upsert never moves last_viewed_at backwards: it used to
	// reset it to 0 on every reply, leaving the thread permanently unread for
	// whoever was most active in it.
	if err := a.Store.Thread().SaveMembership(ctx, &model.ThreadMembership{
		PostID: root.ID, UserID: post.UserID, Following: true, LastViewedAt: post.CreateAt,
	}); err != nil {
		return err
	}

	// The root's author follows their thread from its first reply, unless
	// they already have a membership (and so may have unfollowed on purpose).
	// Without one they never saw their own thread in the inbox, and thread
	// mention counts had nowhere to land.
	if root.UserID != post.UserID {
		if _, err := a.Store.Thread().GetMembership(ctx, root.ID, root.UserID); isNotFound(err) {
			if err := a.Store.Thread().SaveMembership(ctx, &model.ThreadMembership{
				PostID: root.ID, UserID: root.UserID, Following: true, LastViewedAt: root.CreateAt,
			}); err != nil {
				return err
			}
		}
	}

	a.broadcastThreadUpdated(ctx, root.ID)
	return nil
}

// broadcastThreadUpdated sends the thread's current counters to its channel.
// The event carries only the thread: the reply itself already went out as
// "posted", and clients that append a reply on both would show it twice.
func (a *App) broadcastThreadUpdated(ctx context.Context, rootID string) {
	assert.That(rootID != "", "App.broadcastThreadUpdated: empty root id")
	assert.That(a.Hub != nil, "App.broadcastThreadUpdated: nil hub")
	thread, err := a.Store.Thread().Get(ctx, rootID)
	if err != nil {
		slog.Warn("thread_updated: could not read thread", "root_id", rootID, "error", err)
		return
	}
	assert.That(thread != nil && thread.PostID == rootID, "App.broadcastThreadUpdated: store returned another thread")
	a.Hub.Broadcast(&model.WebSocketEvent{
		Event:     model.WebSocketEventThreadUpdated,
		Data:      map[string]any{"thread": thread},
		Broadcast: &model.WebSocketBroadcast{ChannelID: thread.ChannelID},
	})
}

func (a *App) broadcastPostEvent(ctx context.Context, event string, post *model.Post) {
	assert.That(post != nil && post.ID != "", "App.broadcastPostEvent: post has no id")
	assert.That(event != "", "App.broadcastPostEvent: empty event")
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
	assert.That(dataMap["id"] == post.ID, "App.broadcastPostEvent: payload lost the post id")

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: event,
		Data:  dataMap,
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: post.ChannelID,
		},
	}, &pubsub.EventEnvelope{Event: event, PostID: post.ID, ChannelID: post.ChannelID})
}

// requireProvenanceWriter returns a 403 unless authorID may set
// props.provenance: an agent or a bot, or a person's post made by an
// allowlisted client on their behalf (actor non-nil). A person posting for
// themselves may not claim what tools or models produced their words.
func (a *App) requireProvenanceWriter(ctx context.Context, authorID string, actor *model.User) error {
	assert.That(authorID != "", "App.requireProvenanceWriter: empty author id")
	assert.That(a.Store != nil, "App.requireProvenanceWriter: nil store")
	if actor != nil && a.OnBehalfOfAllowed(actor) {
		return nil
	}
	user, err := a.Store.User().Get(ctx, authorID)
	if err != nil {
		return err
	}
	assert.That(user != nil, "App.requireProvenanceWriter: nil user without an error")
	if user.ActorType == model.ActorTypeAgent || user.ActorType == model.ActorTypeBot {
		return nil
	}
	return model.NewForbiddenError("App.Provenance", "only an agent or bot, or an allowlisted client posting on behalf of a user, may set props.provenance")
}
