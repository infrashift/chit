package pubsub

// TopicEvents carries thin real-time event envelopes between processes
// (e.g. chitd → chit-mcp).
const TopicEvents = "chit_events"

// EventEnvelope is the cross-process form of a WebSocket event. PostgreSQL
// NOTIFY payloads are limited to ~8KB, so only identifiers travel; consumers
// fetch full records through the store or API.
type EventEnvelope struct {
	Event        string `json:"event"`
	PostID       string `json:"post_id,omitempty"`
	ChannelID    string `json:"channel_id,omitempty"`
	TeamID       string `json:"team_id,omitempty"`
	TargetUserID string `json:"target_user_id,omitempty"` // set only for user-targeted events
}
