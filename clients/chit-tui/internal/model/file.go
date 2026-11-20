package model

// File mirrors the server's File model: an image that is attached, or that
// waits to be attached, to a post.
type File struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	UploaderID  string `json:"uploader_id"`
	ViaActorID  string `json:"via_actor_id,omitempty"`
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AltText     string `json:"alt_text"`
	PostID      string `json:"post_id,omitempty"`
	CreateAt    int64  `json:"create_at"`
}

// MaxFileBytes is the largest image the server accepts. It equals the
// server's imagecheck.MaxImageBytes.
const MaxFileBytes = 10 << 20
