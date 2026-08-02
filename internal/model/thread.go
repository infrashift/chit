package model

import "net/http"

type Thread struct {
	PostID       string   `json:"post_id"`
	ChannelID    string   `json:"channel_id"`
	ReplyCount   int      `json:"reply_count"`
	LastReplyAt  int64    `json:"last_reply_at"`
	Participants []string `json:"participants"`
}

func (t *Thread) IsValid() *AppError {
	if !IsValidID(t.PostID) {
		return NewAppError("Thread.IsValid", "invalid post_id", "", http.StatusBadRequest)
	}
	if !IsValidID(t.ChannelID) {
		return NewAppError("Thread.IsValid", "invalid channel_id", "", http.StatusBadRequest)
	}
	return nil
}

type ThreadMembership struct {
	PostID             string `json:"post_id"`
	UserID             string `json:"user_id"`
	Following          bool   `json:"following"`
	LastViewedAt       int64  `json:"last_viewed_at"`
	UnreadMentionCount int    `json:"unread_mention_count"`
}

func (tm *ThreadMembership) IsValid() *AppError {
	if !IsValidID(tm.PostID) {
		return NewAppError("ThreadMembership.IsValid", "invalid post_id", "", http.StatusBadRequest)
	}
	if !IsValidID(tm.UserID) {
		return NewAppError("ThreadMembership.IsValid", "invalid user_id", "", http.StatusBadRequest)
	}
	return nil
}

// ThreadResponse wraps a thread with its root post and replies for API
// responses. In a user's thread list, Posts holds the root alone — enough to
// show what the thread is about without a request per thread.
type ThreadResponse struct {
	Thread *Thread `json:"thread"`
	Posts  []*Post `json:"posts"`
	// LastViewedAt is when the caller last read this thread. A thread is
	// unread when Thread.LastReplyAt is later. Zero means never read.
	LastViewedAt int64 `json:"last_viewed_at"`
	// UnreadMentions counts mentions of the caller since they last read it.
	UnreadMentions int `json:"unread_mentions"`
}

// UserThreadList holds a list of threads a user is following.
type UserThreadList struct {
	Threads []*ThreadResponse `json:"threads"`
	Total   int64             `json:"total"`
}
