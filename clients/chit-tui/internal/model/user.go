package model

// Actor types, as the server names them. People, agents and bots are users
// of the same kind; ActorType tells them apart.
const (
	ActorTypeUser  = "user"
	ActorTypeAgent = "agent"
	ActorTypeBot   = "bot"
)

// User mirrors the server's User model.
type User struct {
	ID          string `json:"id"`
	KratosID    string `json:"kratos_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Roles       string `json:"roles"`
	ActorType   string `json:"actor_type,omitempty"`
	CreateAt    int64  `json:"create_at"`
	UpdateAt    int64  `json:"update_at"`
	DeleteAt    int64  `json:"delete_at"`
}

// IsMachine reports whether the user is an agent or a bot, not a person.
func (u *User) IsMachine() bool {
	return u != nil && (u.ActorType == ActorTypeAgent || u.ActorType == ActorTypeBot)
}
