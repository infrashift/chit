package model

// Thread mirrors the server's Thread model.
type Thread struct {
	PostID       string   `json:"post_id"`
	ChannelID    string   `json:"channel_id"`
	ReplyCount   int      `json:"reply_count"`
	LastReplyAt  int64    `json:"last_reply_at"`
	Participants []string `json:"participants"`
}

// ThreadResponse pairs a thread with its root post and replies. The server
// puts the root first in Posts.
type ThreadResponse struct {
	Thread *Thread `json:"thread"`
	Posts  []*Post `json:"posts"`
	// LastViewedAt is when the caller last read this thread. It is unread
	// when Thread.LastReplyAt is later. Zero means never read.
	LastViewedAt int64 `json:"last_viewed_at"`
	// UnreadMentions counts mentions of the caller since they last read it.
	UnreadMentions int `json:"unread_mentions"`
}

// Root returns the thread's root post, or nil when the server sent none.
func (t *ThreadResponse) Root() *Post {
	if t == nil || len(t.Posts) == 0 {
		return nil
	}
	return t.Posts[0]
}

// UserThreadList holds the threads a user follows in a team.
type UserThreadList struct {
	Threads []*ThreadResponse `json:"threads"`
	Total   int64             `json:"total"`
}
