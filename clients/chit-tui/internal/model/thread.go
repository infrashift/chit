package model

// Thread mirrors the server's Thread model.
type Thread struct {
	PostID       string   `json:"post_id"`
	ChannelID    string   `json:"channel_id"`
	ReplyCount   int      `json:"reply_count"`
	LastReplyAt  int64    `json:"last_reply_at"`
	Participants []string `json:"participants"`
}
