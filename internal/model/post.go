package model

// Post mirrors the server's Post model.
type Post struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"`
	UserID    string         `json:"user_id"`
	RootID    string         `json:"root_id,omitempty"`
	Content   string         `json:"content"`
	Type      string         `json:"type,omitempty"`
	Props     map[string]any `json:"props,omitempty"`
	Hashtags  string         `json:"hashtags,omitempty"`
	IsPinned  bool           `json:"is_pinned"`
	EditAt    int64          `json:"edit_at"`
	CreateAt  int64          `json:"create_at"`
	UpdateAt  int64          `json:"update_at"`
	DeleteAt  int64          `json:"delete_at"`
}

// PostList mirrors the server's PostList model.
type PostList struct {
	Order []*Post `json:"order"`
}
