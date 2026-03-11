package model

// Thread mirrors the server's Thread model.
type Thread struct {
	PostID       string   `json:"post_id"`
	ChannelID    string   `json:"channel_id"`
	ReplyCount   int      `json:"reply_count"`
	LastReplyAt  int64    `json:"last_reply_at"`
	Participants []string `json:"participants"`
}

// ThreadMembership mirrors the server's ThreadMembership model.
type ThreadMembership struct {
	PostID             string `json:"post_id"`
	UserID             string `json:"user_id"`
	Following          bool   `json:"following"`
	LastViewedAt       int64  `json:"last_viewed_at"`
	UnreadMentionCount int    `json:"unread_mention_count"`
}

// ThreadResponse contains thread metadata and its posts.
type ThreadResponse struct {
	Thread *Thread `json:"thread"`
	Posts  []*Post `json:"posts"`
}
