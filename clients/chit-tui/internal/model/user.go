package model

// User mirrors the server's User model.
type User struct {
	ID          string `json:"id"`
	KratosID    string `json:"kratos_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Roles       string `json:"roles"`
	CreateAt    int64  `json:"create_at"`
	UpdateAt    int64  `json:"update_at"`
	DeleteAt    int64  `json:"delete_at"`
}
