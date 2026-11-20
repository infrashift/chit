package model

import "github.com/infrashift/chit/clients/chit-tui/internal/assert"

const (
	WebSocketEventPosted          = "posted"
	WebSocketEventPostEdited      = "post_edited"
	WebSocketEventPostDeleted     = "post_deleted"
	WebSocketEventPostPinned      = "post_pinned"
	WebSocketEventPostUnpinned    = "post_unpinned"
	WebSocketEventTyping          = "typing"
	WebSocketEventChannelCreated  = "channel_created"
	WebSocketEventChannelUpdated  = "channel_updated"
	WebSocketEventChannelDeleted  = "channel_deleted"
	WebSocketEventUserAdded       = "user_added"
	WebSocketEventUserRemoved     = "user_removed"
	WebSocketEventThreadUpdated   = "thread_updated"
	WebSocketEventStatusChange    = "status_change"
	WebSocketEventMentioned       = "mentioned"
	WebSocketEventCommandResponse = "command_response"
	// WebSocketEventPostTagsUpdated carries a post's whole tag list after a
	// tag is added or removed.
	WebSocketEventPostTagsUpdated = "post_tags_updated"
	// WebSocketEventStreamStatus tells that a sender starts, continues or
	// ends a streamed reply in a channel.
	WebSocketEventStreamStatus = "stream_status"
	// WebSocketEventTagUpdated carries a renamed tag.
	WebSocketEventTagUpdated = "tag_updated"
	// WebSocketEventTagDeleted carries the ID of a deleted tag. No
	// post_tags_updated follows it.
	WebSocketEventTagDeleted = "tag_deleted"
	// WebSocketEventChannelTagsUpdated carries a channel's whole tag list,
	// as the receiver can see it.
	WebSocketEventChannelTagsUpdated = "channel_tags_updated"
)

// Stream states of a stream_status event.
const (
	StreamStarted  = "started"
	StreamProgress = "progress"
	StreamFinished = "finished"
)

// WebSocketEvent mirrors the server's WebSocketEvent model.
type WebSocketEvent struct {
	Event     string              `json:"event"`
	Data      map[string]any      `json:"data"`
	Broadcast *WebSocketBroadcast `json:"broadcast"`
	Sequence  int64               `json:"seq"`
}

// WebSocketBroadcast defines the delivery scope for a WebSocket event.
type WebSocketBroadcast struct {
	ChannelID string `json:"channel_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// WebSocketMessage is a client-to-server WebSocket message.
type WebSocketMessage struct {
	Action string         `json:"action"`
	Seq    int64          `json:"seq"`
	Data   map[string]any `json:"data"`
}

// AppError mirrors the server's AppError model.
type AppError struct {
	ID            string `json:"id"`
	Message       string `json:"message"`
	DetailedError string `json:"detailed_error,omitempty"`
	StatusCode    int    `json:"status_code"`
	// Reason and Detail carry a policy's own refusal: a short stable code
	// and a text for a person. The server sets them only on
	// channel.membership_refused.
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	assert.That(e != nil, "AppError.Error: nil error")
	return e.Message
}
