package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// Tags (P-CHIT-7, HALO ADR-0009).
//
// Each Tag has an owner and a scope. A private Tag is its owner's alone: only
// the owner sees it, its name, and the items it is on, and only the owner
// applies it, to any channel or post the owner can read. A channel Tag
// belongs to one channel: every member sees it and may apply or remove it on
// that channel and its posts, and nowhere else. Only the owner renames or
// deletes a Tag (a system_admin may also manage the Tags of the system actor,
// which owns the Tags migrated from the old global vocabulary).
//
// PRIVACY IS ENFORCED HERE, not in the store: the store returns Tags of every
// owner, and every read below filters them with model.Tag.VisibleTo before
// they leave the app. An unknown Tag and a Tag the caller cannot see answer
// the same 404, so a Tag ID never confirms that somebody's private Tag
// exists.
//
// The reserved prefix "profile/" belongs to HALO's Profile Tags. A person
// cannot create, rename or apply such a Tag; an allowlisted client acting for
// the owner (X-On-Behalf-Of, actor non-nil) can create and apply one, and
// only as the owner's private Tag.

// TagCreate is a request to create a Tag.
type TagCreate struct {
	Name      string
	Scope     string // private (the default) or channel
	ChannelID string // required for a channel Tag
}

func notFoundTag(tagID string) error {
	return model.NewNotFoundError("App.Tag", tagID)
}

// onBehalf reports whether actor is an allowlisted client acting for the
// request's user.
func (a *App) onBehalf(actor *model.User) bool {
	return actor != nil && a.OnBehalfOfAllowed(actor)
}

// visibleTag returns the Tag if userID can see it: its owner for a private
// Tag, a member of its channel for a channel Tag. Otherwise it is a 404, the
// same as an unknown Tag.
func (a *App) visibleTag(ctx context.Context, tagID, userID string) (*model.Tag, error) {
	tag, err := a.Store.Tag().Get(ctx, tagID)
	if err != nil {
		if isNotFound(err) {
			return nil, notFoundTag(tagID)
		}
		return nil, err
	}
	switch tag.Scope {
	case model.TagScopePrivate:
		if tag.OwnerID != userID {
			return nil, notFoundTag(tagID)
		}
	case model.TagScopeChannel:
		if !a.isChannelMember(ctx, tag.ChannelID, userID) {
			return nil, notFoundTag(tagID)
		}
	default:
		return nil, notFoundTag(tagID)
	}
	return tag, nil
}

// filterTags keeps the Tags userID can see on an item userID can read.
func filterTags(tags []*model.Tag, userID string) []*model.Tag {
	out := make([]*model.Tag, 0, len(tags))
	for _, t := range tags {
		if t.VisibleTo(userID) {
			out = append(out, t)
		}
	}
	return out
}

// requireTagManager returns a 403 unless userID may rename or delete tag:
// its owner, or a system_admin for a Tag of the system actor.
func (a *App) requireTagManager(ctx context.Context, tag *model.Tag, userID string) error {
	if tag.OwnerID == userID {
		return nil
	}
	if tag.OwnerID == model.SystemActorID {
		if err := a.requireSystemAdmin(ctx, userID, "App.Tag"); err == nil {
			return nil
		}
	}
	return model.NewForbiddenError("App.Tag", "only the tag's owner may change it")
}

func reservedRefusal() error {
	return model.NewForbiddenError("App.Tag",
		"profile/ tags are created and applied only by an allowlisted client on the owner's behalf")
}

// CreateTag creates a Tag owned by userID, or returns the one that already
// has the name in its scope (idempotent, as before). actor is the
// allowlisted client when the request came on userID's behalf.
func (a *App) CreateTag(ctx context.Context, req TagCreate, userID string, actor *model.User) (*model.Tag, error) {
	const where = "App.CreateTag"
	if req.Scope == "" {
		req.Scope = model.TagScopePrivate
	}
	if err := a.validateTagCreate(ctx, &req, userID, actor); err != nil {
		return nil, err
	}

	key := model.TagNameKey(req.Name)
	if existing, err := a.Store.Tag().GetByKey(ctx, req.Scope, userID, req.ChannelID, key); err == nil {
		return existing, nil
	} else if !isNotFound(err) {
		return nil, err
	}
	n, err := a.Store.Tag().CountByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	if n >= model.MaxTagsPerOwner {
		return nil, model.NewBadRequestError(where, "you own the most tags allowed (1024); delete one first")
	}
	tag := &model.Tag{Name: req.Name, Scope: req.Scope, ChannelID: req.ChannelID, OwnerID: userID}
	saved, err := a.Store.Tag().Save(ctx, tag)
	if err != nil {
		// A concurrent create of the same name won: answer with it.
		if existing, getErr := a.Store.Tag().GetByKey(ctx, req.Scope, userID, req.ChannelID, key); getErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return saved, nil
}

// validateTagCreate checks a CreateTag request: the name, the scope and its
// channel, and the reservation of profile/ names to allowlisted clients.
func (a *App) validateTagCreate(ctx context.Context, req *TagCreate, userID string, actor *model.User) error {
	const where = "App.CreateTag"
	if err := model.ValidateTagName(req.Name); err != nil {
		return err
	}
	switch req.Scope {
	case model.TagScopePrivate:
		if req.ChannelID != "" {
			return model.NewBadRequestError(where, "a private tag names no channel")
		}
	case model.TagScopeChannel:
		if !model.IsValidID(req.ChannelID) {
			return model.NewBadRequestError(where, "a channel tag needs a channel_id")
		}
		if err := a.requireChannelMember(ctx, req.ChannelID, userID); err != nil {
			return err
		}
	default:
		return model.NewBadRequestError(where, "scope must be private or channel")
	}
	if model.IsReservedTagName(req.Name) {
		if !a.onBehalf(actor) {
			return reservedRefusal()
		}
		if req.Scope != model.TagScopePrivate {
			return model.NewBadRequestError(where, "a profile/ tag is always the owner's private tag")
		}
	}
	return nil
}

// GetVisibleTags lists the Tags userID can see: their private Tags and the
// channel Tags of their channels. A channelID narrows the list to that
// channel's Tags plus userID's private Tags, and needs membership.
func (a *App) GetVisibleTags(ctx context.Context, userID, channelID string) ([]*model.Tag, error) {
	var channelIDs []string
	if channelID != "" {
		if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
			return nil, err
		}
		channelIDs = []string{channelID}
	} else {
		ids, err := a.Store.Channel().GetChannelIDsForUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		channelIDs = ids
	}
	tags, err := a.Store.Tag().GetVisible(ctx, userID, channelIDs)
	if err != nil {
		return nil, err
	}
	// The store already limits the list; filter again so a store bug cannot
	// leak somebody's private Tag.
	return filterTags(tags, userID), nil
}

// RenameTag renames a Tag; its owner only.
func (a *App) RenameTag(ctx context.Context, tagID, name, userID string) (*model.Tag, error) {
	tag, err := a.visibleTag(ctx, tagID, userID)
	if err != nil {
		return nil, err
	}
	if authErr := a.requireTagManager(ctx, tag, userID); authErr != nil {
		return nil, authErr
	}
	if err := model.ValidateTagName(name); err != nil {
		return nil, err
	}
	if tag.IsReserved() || model.IsReservedTagName(name) {
		return nil, reservedRefusal()
	}
	tag.Name = name
	updated, err := a.Store.Tag().Update(ctx, tag)
	if err != nil {
		return nil, err
	}
	a.publishTagEvent(ctx, model.WebSocketEventTagUpdated, updated, map[string]any{"tag": updated})
	return updated, nil
}

// DeleteTag deletes a Tag and every application of it; its owner only.
func (a *App) DeleteTag(ctx context.Context, tagID, userID string) error {
	tag, err := a.visibleTag(ctx, tagID, userID)
	if err != nil {
		return err
	}
	if err := a.requireTagManager(ctx, tag, userID); err != nil {
		return err
	}
	if err := a.Store.Tag().Delete(ctx, tagID); err != nil {
		return err
	}
	a.publishTagEvent(ctx, model.WebSocketEventTagDeleted, tag, map[string]any{"tag_id": tag.ID})
	return nil
}

// DeleteTagsOfUser deletes every Tag targetID owns, with every application
// of them, and clears targetID as the applier of other owners' Tags. It is
// what deleting or anonymizing a person must run (HALO section 3.9). The
// user themselves or a system_admin may run it.
func (a *App) DeleteTagsOfUser(ctx context.Context, targetID, actorID string) error {
	if targetID != actorID {
		if err := a.requireSystemAdmin(ctx, actorID, "App.DeleteTagsOfUser"); err != nil {
			return err
		}
	}
	return a.Store.Tag().DeleteByOwner(ctx, targetID)
}

// applicableTag returns tag if userID may apply or remove it on an item of
// channelID: a private Tag by its owner only, a channel Tag in its own
// channel only. apply also enforces the profile/ rule.
func (a *App) applicableTag(ctx context.Context, tagID, channelID, userID string, actor *model.User, apply bool) (*model.Tag, error) {
	tag, err := a.visibleTag(ctx, tagID, userID)
	if err != nil {
		return nil, err
	}
	if tag.Scope == model.TagScopeChannel && tag.ChannelID != channelID {
		return nil, model.NewBadRequestError("App.Tag", "a channel tag applies only in its own channel")
	}
	if apply && tag.IsReserved() && !a.onBehalf(actor) {
		return nil, reservedRefusal()
	}
	return tag, nil
}

// requireRoomOnItem returns a 400 when the item already holds
// MaxTagsPerItem Tags of tag's class: channel Tags, or the private Tags of
// tag's owner. Counting other owners' private Tags would reveal them.
func requireRoomOnItem(current []*model.Tag, tag *model.Tag) error {
	n := 0
	for _, t := range current {
		if t.ID == tag.ID {
			return nil // already applied: not a new one
		}
		if t.Scope == tag.Scope && (t.Scope == model.TagScopeChannel || t.OwnerID == tag.OwnerID) {
			n++
		}
	}
	if n >= model.MaxTagsPerItem {
		return model.NewBadRequestError("App.Tag", "this item already has the most tags allowed (32)")
	}
	return nil
}

// AddTagToPost applies a Tag to a post. The caller must be able to read the
// post (a member of its channel) and may apply the Tag (see applicableTag).
func (a *App) AddTagToPost(ctx context.Context, postID, tagID, userID string, actor *model.User) error {
	post, err := a.Store.Post().Get(ctx, postID)
	if err != nil {
		return err
	}
	if authErr := a.requireChannelMember(ctx, post.ChannelID, userID); authErr != nil {
		return authErr
	}
	tag, err := a.applicableTag(ctx, tagID, post.ChannelID, userID, actor, true)
	if err != nil {
		return err
	}
	current, err := a.Store.Tag().GetTagsForPost(ctx, postID)
	if err != nil {
		return err
	}
	if err := requireRoomOnItem(current, tag); err != nil {
		return err
	}
	if err := a.Store.Tag().AddTagToPost(ctx, postID, tagID, userID); err != nil {
		return err
	}
	a.publishPostTags(ctx, post, tag)
	return nil
}

// RemoveTagFromPost removes a Tag from a post: any member for a channel Tag,
// the owner for a private one.
func (a *App) RemoveTagFromPost(ctx context.Context, postID, tagID, userID string) error {
	post, err := a.Store.Post().Get(ctx, postID)
	if err != nil {
		return err
	}
	if authErr := a.requireChannelMember(ctx, post.ChannelID, userID); authErr != nil {
		return authErr
	}
	tag, err := a.applicableTag(ctx, tagID, post.ChannelID, userID, nil, false)
	if err != nil {
		return err
	}
	if err := a.Store.Tag().RemoveTagFromPost(ctx, postID, tagID); err != nil {
		return err
	}
	a.publishPostTags(ctx, post, tag)
	return nil
}

// GetTagsForPost returns the Tags on a post that the caller can see.
func (a *App) GetTagsForPost(ctx context.Context, postID, userID string) ([]*model.Tag, error) {
	if err := a.requirePostChannelMember(ctx, postID, userID); err != nil {
		return nil, err
	}
	tags, err := a.Store.Tag().GetTagsForPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	return filterTags(tags, userID), nil
}

// GetTagsForPosts returns the Tags the caller can see on many posts at once,
// restricted to posts in channels the caller belongs to. Posts they cannot
// read are omitted rather than refused, so one inaccessible ID in a batch
// does not fail the request.
//
// Membership is resolved once per distinct channel, not once per post.
func (a *App) GetTagsForPosts(ctx context.Context, messageIDs []string, userID string) (map[string][]*model.Tag, error) {
	out := map[string][]*model.Tag{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	posts, err := a.Store.Post().GetByIDs(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	visible := make([]string, 0, len(posts))
	for _, post := range posts {
		ok, seen := allowed[post.ChannelID]
		if !seen {
			ok = a.isChannelMember(ctx, post.ChannelID, userID)
			allowed[post.ChannelID] = ok
		}
		if ok {
			visible = append(visible, post.ID)
		}
	}
	if len(visible) == 0 {
		return out, nil
	}
	all, err := a.Store.Tag().GetTagsForPosts(ctx, visible)
	if err != nil {
		return nil, err
	}
	for postID, tags := range all {
		if mine := filterTags(tags, userID); len(mine) > 0 {
			out[postID] = mine
		}
	}
	return out, nil
}

// AddTagToChannel applies a Tag to a channel itself.
func (a *App) AddTagToChannel(ctx context.Context, channelID, tagID, userID string, actor *model.User) error {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return err
	}
	tag, err := a.applicableTag(ctx, tagID, channelID, userID, actor, true)
	if err != nil {
		return err
	}
	current, err := a.Store.Tag().GetTagsForChannel(ctx, channelID)
	if err != nil {
		return err
	}
	if err := requireRoomOnItem(current, tag); err != nil {
		return err
	}
	if err := a.Store.Tag().AddTagToChannel(ctx, channelID, tagID, userID); err != nil {
		return err
	}
	a.publishChannelTags(ctx, channelID, tag)
	return nil
}

// RemoveTagFromChannel removes a Tag from a channel.
func (a *App) RemoveTagFromChannel(ctx context.Context, channelID, tagID, userID string) error {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return err
	}
	tag, err := a.applicableTag(ctx, tagID, channelID, userID, nil, false)
	if err != nil {
		return err
	}
	if err := a.Store.Tag().RemoveTagFromChannel(ctx, channelID, tagID); err != nil {
		return err
	}
	a.publishChannelTags(ctx, channelID, tag)
	return nil
}

// GetTagsForChannel returns the Tags on a channel itself that the caller
// can see.
func (a *App) GetTagsForChannel(ctx context.Context, channelID, userID string) ([]*model.Tag, error) {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}
	tags, err := a.Store.Tag().GetTagsForChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return filterTags(tags, userID), nil
}

// visibleTagIDs checks a Tag filter: at most MaxTagFilter distinct IDs, each
// a Tag userID can see. ok is false when one is unknown or invisible: the
// filter then matches nothing, the same answer for both, so it confirms no
// private Tag.
func (a *App) visibleTagIDs(ctx context.Context, tagIDs []string, userID, where string) (ids []string, ok bool, err error) {
	ids = slices.Compact(slices.Sorted(slices.Values(tagIDs)))
	if len(ids) > model.MaxTagFilter {
		return nil, false, model.NewBadRequestError(where, "a filter names at most 8 tags")
	}
	for _, id := range ids {
		if !model.IsValidID(id) {
			return nil, false, model.NewBadRequestError(where, "invalid tag id")
		}
	}
	for _, id := range ids {
		if _, err := a.visibleTag(ctx, id, userID); err != nil {
			if isNotFound(err) {
				return ids, false, nil
			}
			return nil, false, err
		}
	}
	return ids, true, nil
}

// FilterChannelsByTags lists the caller's channels (in teamID, when set)
// that have every Tag in tagIDs, on the channel or on one of its posts. Each
// comes with the channel-level Tags the caller can see.
func (a *App) FilterChannelsByTags(ctx context.Context, userID, teamID string, tagIDs []string) ([]*model.TaggedChannel, error) {
	const where = "App.FilterChannelsByTags"
	out := []*model.TaggedChannel{}
	ids, ok, err := a.visibleTagIDs(ctx, tagIDs, userID, where)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, model.NewBadRequestError(where, "tag_ids is required")
	}
	if !ok {
		return out, nil
	}
	channels, err := a.tagFilterChannels(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}
	byID := map[string]*model.Channel{}
	candidates := make([]string, 0, len(channels))
	for _, c := range channels {
		byID[c.ID] = c
		candidates = append(candidates, c.ID)
	}
	if len(candidates) == 0 {
		return out, nil
	}
	matched, err := a.Store.Tag().FilterChannelIDsByTags(ctx, candidates, ids)
	if err != nil {
		return nil, err
	}
	slices.Sort(matched)
	for _, id := range matched {
		tags, err := a.Store.Tag().GetTagsForChannel(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, &model.TaggedChannel{Channel: byID[id], Tags: filterTags(tags, userID)})
	}
	return out, nil
}

// tagFilterChannels returns the channels FilterChannelsByTags considers:
// userID's channels in teamID (which needs team membership), or all of
// userID's channels when teamID is empty.
func (a *App) tagFilterChannels(ctx context.Context, userID, teamID string) ([]*model.Channel, error) {
	if teamID != "" {
		if authErr := a.requireTeamMember(ctx, teamID, userID); authErr != nil {
			return nil, authErr
		}
		return a.Store.Channel().GetChannelsForUser(ctx, userID, teamID)
	}
	chIDs, err := a.Store.Channel().GetChannelIDsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var channels []*model.Channel
	for _, id := range chIDs {
		if c, getErr := a.Store.Channel().Get(ctx, id); getErr == nil {
			channels = append(channels, c)
		}
	}
	return channels, nil
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

// ─── Events ──────────────────────────────────────────────────────

// tagAudience returns who may hear of a change to changed: its owner for a
// private Tag, every member of channelID for a channel Tag.
func (a *App) tagAudience(ctx context.Context, changed *model.Tag, channelID string) []string {
	if changed.Scope != model.TagScopeChannel {
		return []string{changed.OwnerID}
	}
	ids, err := a.Store.Channel().GetMemberIDs(ctx, channelID)
	if err != nil {
		slog.Warn("tag event: could not read channel members", "channel_id", channelID, "error", err)
		return nil
	}
	return ids
}

// publishPostTags tells each reader who may hear of the change their own
// view of the post's Tags: post_tags_updated, addressed to each user, never
// to the channel as a whole. A private Tag's change reaches its owner's
// sessions only, and only a channel Tag's change goes to the pubsub topic.
// The full list, rather than the one tag that changed, lets a client
// replace what it has without tracking adds and removes.
func (a *App) publishPostTags(ctx context.Context, post *model.Post, changed *model.Tag) {
	all, err := a.Store.Tag().GetTagsForPost(ctx, post.ID)
	if err != nil {
		slog.Warn("post_tags_updated: could not read tags", "post_id", post.ID, "error", err)
		return
	}
	for _, userID := range a.tagAudience(ctx, changed, post.ChannelID) {
		a.Hub.Broadcast(&model.WebSocketEvent{
			Event:     model.WebSocketEventPostTagsUpdated,
			Data:      map[string]any{"post_id": post.ID, "channel_id": post.ChannelID, "tags": filterTags(all, userID)},
			Broadcast: &model.WebSocketBroadcast{UserID: userID},
		})
	}
	a.publishEnvelope(ctx, changed, &pubsub.EventEnvelope{
		Event: model.WebSocketEventPostTagsUpdated, PostID: post.ID, ChannelID: post.ChannelID,
	})
}

// publishChannelTags is publishPostTags for the Tags on a channel itself.
func (a *App) publishChannelTags(ctx context.Context, channelID string, changed *model.Tag) {
	all, err := a.Store.Tag().GetTagsForChannel(ctx, channelID)
	if err != nil {
		slog.Warn("channel_tags_updated: could not read tags", "channel_id", channelID, "error", err)
		return
	}
	for _, userID := range a.tagAudience(ctx, changed, channelID) {
		a.Hub.Broadcast(&model.WebSocketEvent{
			Event:     model.WebSocketEventChannelTagsUpdated,
			Data:      map[string]any{"channel_id": channelID, "tags": filterTags(all, userID)},
			Broadcast: &model.WebSocketBroadcast{UserID: userID},
		})
	}
	a.publishEnvelope(ctx, changed, &pubsub.EventEnvelope{
		Event: model.WebSocketEventChannelTagsUpdated, ChannelID: channelID,
	})
}

// publishTagEvent announces a rename or delete: to the owner for a private
// Tag, to the Tag's channel for a channel Tag.
func (a *App) publishTagEvent(ctx context.Context, event string, tag *model.Tag, data map[string]any) {
	b := &model.WebSocketBroadcast{UserID: tag.OwnerID}
	if tag.Scope == model.TagScopeChannel {
		b = &model.WebSocketBroadcast{ChannelID: tag.ChannelID}
	}
	a.Hub.Broadcast(&model.WebSocketEvent{Event: event, Data: data, Broadcast: b})
	a.publishEnvelope(ctx, tag, &pubsub.EventEnvelope{Event: event, ChannelID: tag.ChannelID})
}

// publishEnvelope sends the thin pubsub envelope for a channel Tag's change
// only. The topic is for out-of-process consumers, the nearest thing Chit
// has to an export, and even "a private tag changed on this post" is not
// theirs to know.
func (a *App) publishEnvelope(ctx context.Context, changed *model.Tag, env *pubsub.EventEnvelope) {
	if changed.Scope != model.TagScopeChannel || a.PubSub == nil {
		return
	}
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	if err := a.PubSub.Publish(ctx, pubsub.TopicEvents, data); err != nil {
		slog.Warn("failed to publish event", "event", env.Event, "error", err)
	}
}
