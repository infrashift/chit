package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// CreateChannel creates a new channel and adds the creator as a member.
// For open channels, all team members are automatically added.
func (a *App) CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	if channel.TeamID != "" {
		if err := a.requireTeamMember(ctx, channel.TeamID, channel.CreatorID); err != nil {
			return nil, err
		}
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

// GetChannel retrieves a channel by ID.
func (a *App) GetChannel(ctx context.Context, id string) (*model.Channel, error) {
	return a.Store.Channel().Get(ctx, id)
}

// UpdateChannel updates a channel on behalf of actorID, who must be a member.
func (a *App) UpdateChannel(ctx context.Context, channel *model.Channel, actorID string) (*model.Channel, error) {
	if err := a.requireChannelMember(ctx, channel.ID, actorID); err != nil {
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
	}, pubsub.EventEnvelope{Event: model.WebSocketEventChannelDeleted, ChannelID: id})
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

	members := []string{userID1, userID2}
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
	if len(userIDs) < 3 || len(userIDs) > 8 {
		return nil, model.NewBadRequestError("App.CreateGroupChannel", "group channels require 3-8 members")
	}
	actorIncluded := false
	for _, uid := range userIDs {
		if uid == actorID {
			actorIncluded = true
			break
		}
	}
	if !actorIncluded {
		return nil, model.NewForbiddenError("App.CreateGroupChannel", "cannot create a group channel that excludes yourself")
	}

	sort.Strings(userIDs)
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

// AddChannelMember adds a user to a channel on behalf of actorID.
// For open channels any team member may join or invite; for private, direct,
// and group channels only existing members may invite.
func (a *App) AddChannelMember(ctx context.Context, channelID, userID, actorID string) (*model.ChannelMember, error) {
	channel, err := a.Store.Channel().Get(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel.Type == model.ChannelOpen && channel.TeamID != "" {
		err = a.requireTeamMember(ctx, channel.TeamID, actorID)
	} else {
		err = a.requireChannelMember(ctx, channelID, actorID)
	}
	if err != nil {
		return nil, err
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
	}, pubsub.EventEnvelope{Event: model.WebSocketEventUserAdded, ChannelID: channelID})

	return saved, nil
}

// RemoveChannelMember removes a user from a channel on behalf of actorID, who
// must be a channel member (leaving is removing yourself).
func (a *App) RemoveChannelMember(ctx context.Context, channelID, userID, actorID string) error {
	if err := a.requireChannelMember(ctx, channelID, actorID); err != nil {
		return err
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
	}, pubsub.EventEnvelope{Event: model.WebSocketEventUserRemoved, ChannelID: channelID})

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

	a.publishEvent(ctx, &model.WebSocketEvent{
		Event: event,
		Data:  dataMap,
		Broadcast: &model.WebSocketBroadcast{
			TeamID: channel.TeamID,
		},
	}, pubsub.EventEnvelope{Event: event, ChannelID: channel.ID, TeamID: channel.TeamID})
}
