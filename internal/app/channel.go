package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/infrashift/chit/internal/model"
)

// CreateChannel creates a new channel and adds the creator as a member.
func (a *App) CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	saved, err := a.Store.Channel().Save(channel)
	if err != nil {
		return nil, err
	}

	member := &model.ChannelMember{
		ChannelID: saved.ID,
		UserID:    saved.CreatorID,
	}
	if _, err := a.Store.Channel().SaveMember(member); err != nil {
		return nil, err
	}

	// Write Keto relation for channel membership
	if err := a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, saved.CreatorID); err != nil {
		// Log but don't fail — Keto may not be available in dev
		fmt.Printf("warning: failed to write keto relation: %v\n", err)
	}

	a.broadcastChannelEvent(model.WebSocketEventChannelCreated, saved)
	return saved, nil
}

// GetChannel retrieves a channel by ID.
func (a *App) GetChannel(id string) (*model.Channel, error) {
	return a.Store.Channel().Get(id)
}

// UpdateChannel updates a channel.
func (a *App) UpdateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	updated, err := a.Store.Channel().Update(channel)
	if err != nil {
		return nil, err
	}

	a.broadcastChannelEvent(model.WebSocketEventChannelUpdated, updated)
	return updated, nil
}

// DeleteChannel soft-deletes a channel.
func (a *App) DeleteChannel(id string) error {
	err := a.Store.Channel().Delete(id, model.GetMillis())
	if err != nil {
		return err
	}

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventChannelDeleted,
		Data:  map[string]any{"channel_id": id},
		Broadcast: &model.WebSocketBroadcast{
			ChannelID: id,
		},
	})
	return nil
}

// GetChannelsForTeam retrieves channels for a team.
func (a *App) GetChannelsForTeam(teamID string, page, perPage int) ([]*model.Channel, error) {
	return a.Store.Channel().GetChannelsForTeam(teamID, page, perPage)
}

// GetChannelsForUser retrieves channels a user is a member of within a team.
func (a *App) GetChannelsForUser(userID, teamID string) ([]*model.Channel, error) {
	return a.Store.Channel().GetChannelsForUser(userID, teamID)
}

// CreateDirectChannel creates a DM channel between two users.
func (a *App) CreateDirectChannel(ctx context.Context, userID1, userID2 string) (*model.Channel, error) {
	members := []string{userID1, userID2}
	sort.Strings(members)

	channel := &model.Channel{
		Name:        strings.Join(members, "__"),
		DisplayName: strings.Join(members, ", "),
		Type:        model.ChannelDirect,
		CreatorID:   userID1,
	}

	saved, err := a.Store.Channel().SaveDirectChannel(channel, members)
	if err != nil {
		return nil, err
	}

	for _, uid := range members {
		_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, uid)
	}

	return saved, nil
}

// CreateGroupChannel creates a GM channel among multiple users.
func (a *App) CreateGroupChannel(ctx context.Context, userIDs []string) (*model.Channel, error) {
	if len(userIDs) < 3 || len(userIDs) > 8 {
		return nil, model.NewBadRequestError("App.CreateGroupChannel", "group channels require 3-8 members")
	}

	sort.Strings(userIDs)
	channel := &model.Channel{
		Name:        strings.Join(userIDs, "__"),
		DisplayName: fmt.Sprintf("Group (%d members)", len(userIDs)),
		Type:        model.ChannelGroup,
		CreatorID:   userIDs[0],
	}

	saved, err := a.Store.Channel().SaveDirectChannel(channel, userIDs)
	if err != nil {
		return nil, err
	}

	for _, uid := range userIDs {
		_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, saved.ID, model.KetoRelationMember, uid)
	}

	return saved, nil
}

// AddChannelMember adds a user to a channel.
func (a *App) AddChannelMember(ctx context.Context, channelID, userID string) (*model.ChannelMember, error) {
	member := &model.ChannelMember{
		ChannelID: channelID,
		UserID:    userID,
	}

	saved, err := a.Store.Channel().SaveMember(member)
	if err != nil {
		return nil, err
	}

	_ = a.WriteKetoRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID)

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventUserAdded,
		Data: map[string]any{
			"channel_id": channelID,
			"user_id":    userID,
		},
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	})

	return saved, nil
}

// RemoveChannelMember removes a user from a channel.
func (a *App) RemoveChannelMember(ctx context.Context, channelID, userID string) error {
	if err := a.Store.Channel().RemoveMember(channelID, userID); err != nil {
		return err
	}

	_ = a.DeleteKetoRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID)

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: model.WebSocketEventUserRemoved,
		Data: map[string]any{
			"channel_id": channelID,
			"user_id":    userID,
		},
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	})

	return nil
}

// GetChannelMembers retrieves members of a channel.
func (a *App) GetChannelMembers(channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	return a.Store.Channel().GetMembers(channelID, page, perPage)
}

// UpdateChannelLastViewedAt marks a channel as viewed by a user.
func (a *App) UpdateChannelLastViewedAt(channelID, userID string) error {
	return a.Store.Channel().UpdateLastViewedAt(channelID, userID, model.GetMillis())
}

func (a *App) broadcastChannelEvent(event string, channel *model.Channel) {
	data, _ := json.Marshal(channel)
	var dataMap map[string]any
	json.Unmarshal(data, &dataMap)

	a.Hub.Broadcast(&model.WebSocketEvent{
		Event: event,
		Data:  dataMap,
		Broadcast: &model.WebSocketBroadcast{
			TeamID: channel.TeamID,
		},
	})
}
