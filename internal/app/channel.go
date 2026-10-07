package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// CreateChannel creates an open or private team channel and adds the creator
// as a member. For open channels, all team members are automatically added.
//
// Direct and group channels are NOT created here: they have no team, and their
// name is the sorted member list that CreateDirectChannel and
// CreateGroupChannel look them up by. Accepting them here let a caller create
// "alice__bob" as its only member, add the pair, and then receive every DM
// they later sent each other.
func (a *App) CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	if channel.Type != model.ChannelOpen && channel.Type != model.ChannelPrivate {
		return nil, model.NewBadRequestError("App.CreateChannel",
			"type must be O or P; use /channels/direct or /channels/group for direct messages")
	}
	if channel.TeamID == "" {
		return nil, model.NewBadRequestError("App.CreateChannel", "team_id is required")
	}
	if err := a.requireTeamMember(ctx, channel.TeamID, channel.CreatorID); err != nil {
		return nil, err
	}

	saved, err := a.Store.Channel().Save(ctx, channel)
	if err != nil {
		return nil, err
	}

	member := &model.ChannelMember{
		ChannelID: saved.ID,
		UserID:    saved.CreatorID,
	}
	if _, err := a.Store.Channel().SaveMember(ctx, member); err != nil {
		return nil, err
	}
	a.Hub.NotifyMembershipChanged(saved.CreatorID, saved.ID, true)

	// Write Keto relation for channel membership
	if err := a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, saved.CreatorID); err != nil {
		// Log but don't fail — Keto may not be available in dev
		fmt.Printf("warning: failed to write keto relation: %v\n", err)
	}

	// For open channels, auto-add all team members so the channel appears
	// in every member's sidebar via GetChannelsForUser.
	if saved.Type == model.ChannelOpen && saved.TeamID != "" {
		teamMembers, tmErr := a.Store.Team().GetMembers(ctx, saved.TeamID, 0, 10000)
		if tmErr == nil {
			for _, tm := range teamMembers {
				if tm.UserID == saved.CreatorID {
					continue // already added above
				}
				_, _ = a.Store.Channel().SaveMember(ctx, &model.ChannelMember{
					ChannelID: saved.ID,
					UserID:    tm.UserID,
				})
				a.Hub.NotifyMembershipChanged(tm.UserID, saved.ID, true)
				_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, tm.UserID)
			}
		}
	}

	a.broadcastChannelEvent(ctx, model.WebSocketEventChannelCreated, saved)
	return saved, nil
}

// GetChannel retrieves a channel on behalf of actorID. Members may read any
// channel; anyone on the team may read an open channel, which is what lets
// them find it to join. Private, direct and group channels are hidden from
// non-members: their header and purpose are private, and a DM's name is the
// two participants' IDs.
func (a *App) GetChannel(ctx context.Context, id, actorID string) (*model.Channel, error) {
	channel, err := a.Store.Channel().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.isChannelMember(ctx, id, actorID) {
		return channel, nil
	}
	if channel.Type == model.ChannelOpen && channel.TeamID != "" {
		if err := a.requireTeamMember(ctx, channel.TeamID, actorID); err != nil {
			return nil, err
		}
		return channel, nil
	}
	return nil, model.NewForbiddenError("App.GetChannel", "not a member of this channel")
}

// ChannelPatch is a partial channel update. A nil field is left unchanged; a
// non-nil one is applied even when empty, so a header can be cleared. The
// name is not patchable: it is the channel's URL slug, and for direct and
// group channels it is the member list their lookup depends on.
type ChannelPatch struct {
	DisplayName *string
	Header      *string
	Purpose     *string
}

// UpdateChannel applies patch on behalf of actorID, who must be a member.
func (a *App) UpdateChannel(ctx context.Context, id string, patch ChannelPatch, actorID string) (*model.Channel, error) {
	if err := a.requireChannelMember(ctx, id, actorID); err != nil {
		return nil, err
	}
	channel, err := a.Store.Channel().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if patch.DisplayName != nil {
		channel.DisplayName = *patch.DisplayName
	}
	if patch.Header != nil {
		channel.Header = *patch.Header
	}
	if patch.Purpose != nil {
		channel.Purpose = *patch.Purpose
	}
	if err := channel.IsValid(); err != nil {
		return nil, err
	}

	updated, err := a.Store.Channel().Update(ctx, channel)
	if err != nil {
		return nil, err
	}

	a.broadcastChannelEvent(ctx, model.WebSocketEventChannelUpdated, updated)
	return updated, nil
}

// DeleteChannel soft-deletes a channel on behalf of actorID, who must be the
// channel creator or a system admin.
func (a *App) DeleteChannel(ctx context.Context, id, actorID string) error {
	channel, err := a.Store.Channel().Get(ctx, id)
	if err != nil {
		return err
	}
	if channel.CreatorID != actorID {
		if user, uerr := a.Store.User().Get(ctx, actorID); uerr != nil || !user.IsSystemAdmin() {
			return model.NewForbiddenError("App.DeleteChannel", "only the channel creator may delete a channel")
		}
	}

	if err := a.Store.Channel().Delete(ctx, id, model.GetMillis()); err != nil {
		return err
	}

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: model.WebSocketEventChannelDeleted,
		Data:  map[string]any{"channel_id": id},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: id,
		},
	}, &pubsub.EventEnvelope{Event: model.WebSocketEventChannelDeleted, ChannelID: id})
	return nil
}

// GetChannelsForTeam retrieves channels for a team, requiring the caller to be
// a team member.
func (a *App) GetChannelsForTeam(ctx context.Context, teamID, userID string, page, perPage int) ([]*model.Channel, error) {
	if err := a.requireTeamMember(ctx, teamID, userID); err != nil {
		return nil, err
	}
	return a.Store.Channel().GetChannelsForTeam(ctx, teamID, page, perPage)
}

// GetChannelsForUser retrieves channels a user is a member of within a team.
func (a *App) GetChannelsForUser(ctx context.Context, userID, teamID string) ([]*model.Channel, error) {
	return a.Store.Channel().GetChannelsForUser(ctx, userID, teamID)
}

// GetDirectChannelsForUser retrieves DM and group channels for a user.
func (a *App) GetDirectChannelsForUser(ctx context.Context, userID string) ([]*model.Channel, error) {
	return a.Store.Channel().GetDirectChannelsForUser(ctx, userID)
}

// CreateDirectChannel creates a DM channel between two users on behalf of
// actorID, who must be one of them. If a DM already exists between them, it
// returns the existing channel.
func (a *App) CreateDirectChannel(ctx context.Context, actorID, userID1, userID2 string) (*model.Channel, error) {
	if actorID != userID1 && actorID != userID2 {
		return nil, model.NewForbiddenError("App.CreateDirectChannel", "cannot create a direct channel for other users")
	}
	if userID1 == userID2 {
		return nil, model.NewBadRequestError("App.CreateDirectChannel", "a direct channel needs two different users")
	}

	members := []string{userID1, userID2}
	if err := a.requireUsersExist(ctx, "App.CreateDirectChannel", members); err != nil {
		return nil, err
	}
	sort.Strings(members)
	channelName := strings.Join(members, "__")

	// Idempotent: return existing DM if it exists
	if existing, err := a.Store.Channel().GetDirectChannelByName(ctx, channelName); err == nil {
		return existing, nil
	}

	channel := &model.Channel{
		Name:        channelName,
		DisplayName: strings.Join(members, ", "),
		Type:        model.ChannelDirect,
		CreatorID:   userID1,
	}

	saved, err := a.Store.Channel().SaveDirectChannel(ctx, channel, members)
	if err != nil {
		return nil, err
	}

	for _, uid := range members {
		a.Hub.NotifyMembershipChanged(uid, saved.ID, true)
		_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, uid)
	}

	a.broadcastChannelEvent(ctx, model.WebSocketEventChannelCreated, saved)
	return saved, nil
}

// CreateGroupChannel creates a GM channel among multiple users on behalf of
// actorID, who must be one of them. If a GM already exists with the same
// members, it returns the existing channel.
func (a *App) CreateGroupChannel(ctx context.Context, actorID string, userIDs []string) (*model.Channel, error) {
	// Sorted and de-duplicated into a fresh slice: the sorted list is the
	// channel's identity, and the caller's slice is left as it was. A repeated
	// ID would otherwise pass the size check and then fail the insert on the
	// primary key, as a 500.
	userIDs = slices.Compact(slices.Sorted(slices.Values(userIDs)))
	if len(userIDs) < 3 || len(userIDs) > 8 {
		return nil, model.NewBadRequestError("App.CreateGroupChannel", "group channels require 3-8 distinct members")
	}
	if !slices.Contains(userIDs, actorID) {
		return nil, model.NewForbiddenError("App.CreateGroupChannel", "cannot create a group channel that excludes yourself")
	}
	if err := a.requireUsersExist(ctx, "App.CreateGroupChannel", userIDs); err != nil {
		return nil, err
	}

	channelName := strings.Join(userIDs, "__")

	// Idempotent: return existing GM if it exists
	if existing, err := a.Store.Channel().GetDirectChannelByName(ctx, channelName); err == nil {
		return existing, nil
	}

	channel := &model.Channel{
		Name:        channelName,
		DisplayName: fmt.Sprintf("Group (%d members)", len(userIDs)),
		Type:        model.ChannelGroup,
		CreatorID:   userIDs[0],
	}

	saved, err := a.Store.Channel().SaveDirectChannel(ctx, channel, userIDs)
	if err != nil {
		return nil, err
	}

	for _, uid := range userIDs {
		a.Hub.NotifyMembershipChanged(uid, saved.ID, true)
		_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, uid)
	}

	a.broadcastChannelEvent(ctx, model.WebSocketEventChannelCreated, saved)
	return saved, nil
}

// requireUsersExist returns a 400 unless every ID names an existing user.
// Without it an unknown ID reaches the member insert and fails its foreign key
// as a 500.
func (a *App) requireUsersExist(ctx context.Context, op string, ids []string) error {
	for _, id := range ids {
		if !model.IsValidID(id) {
			return model.NewBadRequestError(op, "invalid user id")
		}
	}
	found, err := a.Store.User().GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if len(found) != len(ids) {
		return model.NewBadRequestError(op, "unknown user id")
	}
	return nil
}

// AddChannelMember adds a user to a channel on behalf of actorID.
// For open channels any team member may join or invite; for private channels
// only existing members may invite. Either way the user being added must be
// on the channel's team.
//
// Direct and group channels have a fixed membership: their name IS the member
// list, and adding a third person to a DM would hand them its whole history.
func (a *App) AddChannelMember(ctx context.Context, channelID, userID, actorID string) (*model.ChannelMember, error) {
	channel, err := a.Store.Channel().Get(ctx, channelID)
	if err != nil {
		return nil, err
	}
	switch channel.Type {
	case model.ChannelDirect, model.ChannelGroup:
		return nil, model.NewForbiddenError("App.AddChannelMember",
			"members cannot be added to a direct or group channel")
	case model.ChannelOpen:
		err = a.requireTeamMember(ctx, channel.TeamID, actorID)
	default:
		err = a.requireChannelMember(ctx, channelID, actorID)
	}
	if err != nil {
		return nil, err
	}
	if a.requireTeamMember(ctx, channel.TeamID, userID) != nil {
		return nil, model.NewBadRequestError("App.AddChannelMember", "user is not a member of this channel's team")
	}
	// Re-adding is a no-op: say so without announcing a join that did not happen.
	if existing, lookupErr := a.Store.Channel().GetMember(ctx, channelID, userID); lookupErr == nil {
		return existing, nil
	}

	member := &model.ChannelMember{
		ChannelID: channelID,
		UserID:    userID,
	}

	saved, err := a.Store.Channel().SaveMember(ctx, member)
	if err != nil {
		return nil, err
	}
	a.Hub.NotifyMembershipChanged(userID, channelID, true)

	_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID)

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: model.WebSocketEventUserAdded,
		Data: map[string]any{
			"channel_id": channelID,
			"user_id":    userID,
		},
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	}, &pubsub.EventEnvelope{Event: model.WebSocketEventUserAdded, ChannelID: channelID})

	return saved, nil
}

// RemoveChannelMember removes a user from a channel on behalf of actorID.
//
// Leaving is always permitted for a member removing themselves. Removing
// somebody else additionally requires system-admin rights: /kick is declared
// admin-only in the CUE command registry, but that restriction lives in the
// command layer and is bypassed entirely by a direct REST call, so it is
// enforced here too.
func (a *App) RemoveChannelMember(ctx context.Context, channelID, userID, actorID string) error {
	if err := a.requireChannelMember(ctx, channelID, actorID); err != nil {
		return err
	}
	if userID != actorID {
		if user, uerr := a.Store.User().Get(ctx, actorID); uerr != nil || !user.IsSystemAdmin() {
			return model.NewForbiddenError("App.RemoveChannelMember",
				"only a system admin may remove another member")
		}
	}

	if err := a.Store.Channel().RemoveMember(ctx, channelID, userID); err != nil {
		return err
	}
	a.Hub.NotifyMembershipChanged(userID, channelID, false)

	_ = a.DeleteKetoRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID)

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: model.WebSocketEventUserRemoved,
		Data: map[string]any{
			"channel_id": channelID,
			"user_id":    userID,
		},
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	}, &pubsub.EventEnvelope{Event: model.WebSocketEventUserRemoved, ChannelID: channelID})

	return nil
}

// GetChannelMembers retrieves members of a channel, requiring the caller to be
// a member.
func (a *App) GetChannelMembers(ctx context.Context, channelID, userID string, page, perPage int) ([]*model.ChannelMember, error) {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return nil, err
	}
	return a.Store.Channel().GetMembers(ctx, channelID, page, perPage)
}

// UpdateChannelLastViewedAt marks a channel as viewed by a user.
func (a *App) UpdateChannelLastViewedAt(ctx context.Context, channelID, userID string) error {
	if err := a.requireChannelMember(ctx, channelID, userID); err != nil {
		return err
	}
	return a.Store.Channel().UpdateLastViewedAt(ctx, channelID, userID, model.GetMillis())
}

func (a *App) broadcastChannelEvent(ctx context.Context, event string, channel *model.Channel) {
	data, err := json.Marshal(channel)
	if err != nil {
		slog.Error("failed to marshal channel for broadcast", "channel_id", channel.ID, "error", err)
		return
	}
	var dataMap map[string]any
	if err := json.Unmarshal(data, &dataMap); err != nil {
		slog.Error("failed to build broadcast payload", "channel_id", channel.ID, "error", err)
		return
	}

	// Open team channels are announced to the whole team; private, direct,
	// and group channels only to their members.
	broadcast := &model.WebSocketBroadcast{ChannelID: channel.ID}
	if channel.Type == model.ChannelOpen && channel.TeamID != "" {
		broadcast = &model.WebSocketBroadcast{TeamID: channel.TeamID}
	}

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event:     event,
		Data:      dataMap,
		Broadcast: broadcast,
	}, &pubsub.EventEnvelope{Event: event, ChannelID: channel.ID, TeamID: channel.TeamID})
}
