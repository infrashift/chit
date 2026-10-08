package app

import (
	"context"
	"log/slog"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/pubsub"
)

// maxMentionNames caps how many distinct @names one post resolves. Each name
// used to cost two queries inside the post request; a post of a few hundred
// @names was a few hundred round trips.
const maxMentionNames = 50

// processMentions resolves @username mentions in a post to user IDs.
// It handles @all/@channel by collecting all channel members.
// Self-mentions and non-channel-members are filtered out.
func (a *App) processMentions(ctx context.Context, post *model.Post) ([]string, error) {
	usernames := model.ParseMentions(post.Content)
	if len(usernames) == 0 {
		return nil, nil
	}

	broadcastAll := false
	individual := make([]string, 0, len(usernames))
	for _, u := range usernames {
		if u == "all" || u == "channel" {
			broadcastAll = true
		} else if len(individual) < maxMentionNames {
			individual = append(individual, u)
		}
	}

	mentioned := make(map[string]struct{})
	if broadcastAll {
		ids, err := a.Store.Channel().GetMemberIDs(ctx, post.ChannelID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			mentioned[id] = struct{}{}
		}
	}
	if len(individual) > 0 {
		// One query resolves the names and checks membership together.
		ids, err := a.Store.Channel().GetMemberIDsByUsernames(ctx, post.ChannelID, individual)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			mentioned[id] = struct{}{}
		}
	}
	delete(mentioned, post.UserID)

	if len(mentioned) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(mentioned))
	for id := range mentioned {
		result = append(result, id)
	}
	slog.Debug("processMentions: resolved mentions",
		"count", len(result), "channel_id", post.ChannelID, "post_user_id", post.UserID)
	return result, nil
}

// notifyMentionedUsers increments mention counters, in one statement per
// table rather than one per user, and sends each user a targeted WebSocket
// event.
func (a *App) notifyMentionedUsers(ctx context.Context, post *model.Post, userIDs []string) {
	if err := a.Store.Channel().IncrementMentionCounts(ctx, post.ChannelID, userIDs); err != nil {
		slog.Warn("notifyMentionedUsers: failed to increment channel mention counts",
			"channel_id", post.ChannelID, "error", err)
	}
	// Only followers have a thread membership to count against; the store
	// skips the rest.
	if post.RootID != "" {
		if err := a.Store.Thread().IncrementMentionCounts(ctx, post.RootID, userIDs); err != nil {
			slog.Warn("notifyMentionedUsers: failed to increment thread mention counts",
				"root_id", post.RootID, "error", err)
		}
	}

	for _, userID := range userIDs {
		a.publishEvent(ctx, &model.WebSocketEvent{
			Event: model.WebSocketEventMentioned,
			Data: map[string]any{
				"post_id":    post.ID,
				"channel_id": post.ChannelID,
				"user_id":    post.UserID,
			},
			Broadcast: &model.WebSocketBroadcast{
				UserID: userID,
			},
		}, &pubsub.EventEnvelope{
			Event:        model.WebSocketEventMentioned,
			PostID:       post.ID,
			ChannelID:    post.ChannelID,
			TargetUserID: userID,
		})
	}
}
