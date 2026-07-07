package model

const (
	TeamOpen       = "O"
	TeamInviteOnly = "I"
)

// Team mirrors the server's Team model.
type Team struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	CreatorID   string `json:"creator_id"`
	CreateAt    int64  `json:"create_at"`
	UpdateAt    int64  `json:"update_at"`
	DeleteAt    int64  `json:"delete_at"`
}
