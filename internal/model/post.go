package model

import "net/http"

const PostMaxContentSize = 65535

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

func (p *Post) IsValid() *AppError {
	if !IsValidID(p.ID) {
		return NewAppError("Post.IsValid", "invalid post id", "", http.StatusBadRequest)
	}
	if !IsValidID(p.ChannelID) {
		return NewAppError("Post.IsValid", "invalid channel_id", "", http.StatusBadRequest)
	}
	if !IsValidID(p.UserID) {
		return NewAppError("Post.IsValid", "invalid user_id", "", http.StatusBadRequest)
	}
	if p.RootID != "" && !IsValidID(p.RootID) {
		return NewAppError("Post.IsValid", "invalid root_id", "", http.StatusBadRequest)
	}
	if p.Content == "" {
		return NewAppError("Post.IsValid", "content is required", "", http.StatusBadRequest)
	}
	if len(p.Content) > PostMaxContentSize {
		return NewAppError("Post.IsValid", "content exceeds maximum size", "", http.StatusBadRequest)
	}
	if p.CreateAt == 0 {
		return NewAppError("Post.IsValid", "create_at is required", "", http.StatusBadRequest)
	}
	return nil
}

func (p *Post) PreSave() {
	if p.ID == "" {
		p.ID = NewID()
	}
	now := GetMillis()
	if p.CreateAt == 0 {
		p.CreateAt = now
	}
	p.UpdateAt = now
	if p.Props == nil {
		p.Props = make(map[string]any)
	}
}

func (p *Post) PreUpdate() {
	p.UpdateAt = GetMillis()
}

// PostList holds an ordered list of posts along with an ordering slice.
type PostList struct {
	Order []*Post `json:"order"`
}
