package app

import (
	"context"
	"regexp"

	"github.com/infrashift/chit/internal/model"
)

var streamIDRe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

// SendStreamStatus relays a stream_status event to the members of a channel:
// sender is producing a reply there (started), is still producing it
// (progress), or is done (finished). HALO's halod sends it while it streams
// a Profile's reply to the requester, so the other members see activity.
//
// The sender must be a member of the channel. A non-nil actor is the
// allowlisted client sending on behalf of sender, and is named in the event.
//
// NOTHING IS STORED: no post, no channel counter, no pubsub envelope. The
// event goes to the WebSocket clients connected to this chitd and nowhere
// else, like typing.
func (a *App) SendStreamStatus(ctx context.Context, channelID string, sender, actor *model.User, status, streamID string) error {
	const where = "App.SendStreamStatus"
	switch status {
	case model.StreamStatusStarted, model.StreamStatusProgress, model.StreamStatusFinished:
	default:
		return model.NewBadRequestError(where, "status must be started, progress or finished")
	}
	if streamID != "" && !streamIDRe.MatchString(streamID) {
		return model.NewBadRequestError(where, "stream_id must be 1 to 64 letters, digits or ._:-")
	}
	if !model.IsValidID(channelID) {
		return model.NewBadRequestError(where, "invalid channel id")
	}
	if err := a.requireChannelMember(ctx, channelID, sender.ID); err != nil {
		return err
	}

	data := map[string]any{
		"channel_id": channelID,
		"user_id":    sender.ID,
		"status":     status,
	}
	if streamID != "" {
		data["stream_id"] = streamID
	}
	if actor != nil {
		data["actor_id"] = actor.ID
	}
	a.Hub.Broadcast(&model.WebSocketEvent{
		Event:     model.WebSocketEventStreamStatus,
		Data:      data,
		Broadcast: &model.WebSocketBroadcast{ChannelID: channelID},
	})
	return nil
}
