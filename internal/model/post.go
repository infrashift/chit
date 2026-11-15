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
	// FileIDs are the attached files, in order. They are set when the post
	// is created and do not change; deleting the post deletes them.
	FileIDs  []string `json:"file_ids,omitempty"`
	EditAt   int64    `json:"edit_at"`
	CreateAt int64    `json:"create_at"`
	UpdateAt int64    `json:"update_at"`
	DeleteAt int64    `json:"delete_at"`
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
	// A post of attachments alone needs no text.
	if p.Content == "" && len(p.FileIDs) == 0 {
		return NewAppError("Post.IsValid", "content is required", "", http.StatusBadRequest)
	}
	if len(p.FileIDs) > FileMaxIDsPerPost {
		return NewAppError("Post.IsValid", "a post has at most 4 file_ids", "", http.StatusBadRequest)
	}
	for _, id := range p.FileIDs {
		if !IsValidID(id) {
			return NewAppError("Post.IsValid", "invalid file id", "", http.StatusBadRequest)
		}
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

// PostCursor is a position in update order: the (update_at, id) of the last
// post read. update_at alone is not a position, since many posts can share
// one, and resuming from it either re-reads or skips them.
type PostCursor struct {
	UpdateAt int64
	ID       string
}

// PostSearch is a scoped post search. ChannelIDs is the scope and is
// required: an empty scope matches nothing. Terms is matched as a substring
// of the content; every tag in TagIDs must be on a post; AuthorID, when set,
// limits results to one author. Page is zero-based.
type PostSearch struct {
	Terms      string
	TagIDs     []string
	ChannelIDs []string
	AuthorID   string
	Page       int
	PerPage    int
}

// PostList holds an ordered list of posts along with an ordering slice.
type PostList struct {
	Order []*Post `json:"order"`
}
