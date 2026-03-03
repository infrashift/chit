package model

const (
	WebSocketEventPosted         = "posted"
	WebSocketEventPostEdited     = "post_edited"
	WebSocketEventPostDeleted    = "post_deleted"
	WebSocketEventPostPinned     = "post_pinned"
	WebSocketEventPostUnpinned   = "post_unpinned"
	WebSocketEventTyping         = "typing"
	WebSocketEventChannelCreated = "channel_created"
	WebSocketEventChannelUpdated = "channel_updated"
	WebSocketEventChannelDeleted = "channel_deleted"
	WebSocketEventUserAdded      = "user_added"
	WebSocketEventUserRemoved    = "user_removed"
	WebSocketEventThreadUpdated  = "thread_updated"
	WebSocketEventStatusChange   = "status_change"
)

// WebSocketEvent is sent to clients over WebSocket connections.
type WebSocketEvent struct {
	Event     string         `json:"event"`
	Data      map[string]any `json:"data"`
	Broadcast *WebSocketBroadcast `json:"broadcast"`
	Sequence  int64          `json:"seq"`
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
