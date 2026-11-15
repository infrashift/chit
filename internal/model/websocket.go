package model

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
	WebSocketEventMentioned       = "mentioned"
	WebSocketEventCommandResponse = "command_response"
	// WebSocketEventPostTagsUpdated carries a post's whole tag list after a
	// tag is added or removed.
	WebSocketEventPostTagsUpdated = "post_tags_updated"
	// WebSocketEventChannelTagsUpdated carries a channel's own Tag list, as
	// its reader sees it, after a Tag is applied to or removed from it.
	WebSocketEventChannelTagsUpdated = "channel_tags_updated"
	// WebSocketEventTagUpdated and WebSocketEventTagDeleted announce a
	// rename or a delete: to the owner for a private Tag, to the channel for
	// a channel Tag.
	WebSocketEventTagUpdated = "tag_updated"
	WebSocketEventTagDeleted = "tag_deleted"
	// WebSocketEventStreamStatus tells a channel that a member, or an agent
	// working for one, is producing a reply. chitd relays it and never
	// stores it.
	WebSocketEventStreamStatus = "stream_status"
)

// Stream status values.
const (
	StreamStatusStarted  = "started"
	StreamStatusProgress = "progress"
	StreamStatusFinished = "finished"
)

// WebSocketEvent is sent to clients over WebSocket connections.
type WebSocketEvent struct {
	Event     string              `json:"event"`
	Data      map[string]any      `json:"data"`
	Broadcast *WebSocketBroadcast `json:"broadcast"`
	Sequence  int64               `json:"seq"`
}

// WebSocketBroadcast controls which clients receive a WebSocket event.
type WebSocketBroadcast struct {
	ChannelID string `json:"channel_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// WebSocketMessage is a message received from clients over WebSocket.
type WebSocketMessage struct {
	Action string         `json:"action"`
	Seq    int64          `json:"seq"`
	Data   map[string]any `json:"data"`
}
