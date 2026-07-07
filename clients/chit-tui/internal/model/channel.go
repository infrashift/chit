package model

const (
	ChannelOpen    = "O"
	ChannelPrivate = "P"
	ChannelDirect  = "D"
	ChannelGroup   = "G"
)

// Channel mirrors the server's Channel model.
type Channel struct {
	ID            string `json:"id"`
	TeamID        string `json:"team_id,omitempty"`
	CreatorID     string `json:"creator_id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Header        string `json:"header"`
	Purpose       string `json:"purpose"`
	Type          string `json:"type"`
	TotalMsgCount int64  `json:"total_msg_count"`
	LastPostAt    int64  `json:"last_post_at"`
	CreateAt      int64  `json:"create_at"`
	UpdateAt      int64  `json:"update_at"`
	DeleteAt      int64  `json:"delete_at"`
}

// ChannelMember mirrors the server's ChannelMember model.
type ChannelMember struct {
	ChannelID    string         `json:"channel_id"`
	UserID       string         `json:"user_id"`
	Roles        string         `json:"roles"`
	LastViewedAt int64          `json:"last_viewed_at"`
	MsgCount     int64          `json:"msg_count"`
	MentionCount int64          `json:"mention_count"`
	NotifyProps  map[string]any `json:"notify_props"`
	CreateAt     int64          `json:"create_at"`
}
