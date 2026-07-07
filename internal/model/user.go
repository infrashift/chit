package model

import (
	"net/http"
	"regexp"
	"strings"
)

var validUsernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}[a-z0-9]$`)

// Actor types. Humans, AI agents, and bots are all equal users; actor_type
// records what kind of actor a user is for audit and policy purposes.
const (
	ActorTypeUser  = "user"
	ActorTypeAgent = "agent"
	ActorTypeBot   = "bot"
)

type User struct {
	ID          string `json:"id"`
	KratosID    string `json:"kratos_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Roles       string `json:"roles"`
	ActorType   string `json:"actor_type"`
	CreateAt    int64  `json:"create_at"`
	UpdateAt    int64  `json:"update_at"`
	DeleteAt    int64  `json:"delete_at"`
}

func (u *User) IsValid() *AppError {
	if !IsValidID(u.ID) {
		return NewAppError("User.IsValid", "invalid user id", "", http.StatusBadRequest)
	}
	if u.KratosID == "" {
		return NewAppError("User.IsValid", "kratos_id is required", "", http.StatusBadRequest)
	}
	if !validUsernameRe.MatchString(u.Username) {
		return NewAppError("User.IsValid", "invalid username", "", http.StatusBadRequest)
	}
	if len(u.DisplayName) == 0 || len(u.DisplayName) > 100 {
		return NewAppError("User.IsValid", "display_name must be 1–100 characters", "", http.StatusBadRequest)
	}
	if len(u.Email) == 0 || len(u.Email) > 128 || !strings.Contains(u.Email, "@") {
		return NewAppError("User.IsValid", "invalid email", "", http.StatusBadRequest)
	}
	if u.CreateAt == 0 {
		return NewAppError("User.IsValid", "create_at is required", "", http.StatusBadRequest)
	}
	if u.UpdateAt == 0 {
		return NewAppError("User.IsValid", "update_at is required", "", http.StatusBadRequest)
	}
	switch u.ActorType {
	case ActorTypeUser, ActorTypeAgent, ActorTypeBot:
	default:
		return NewAppError("User.IsValid", "invalid actor_type", "", http.StatusBadRequest)
	}
	return nil
}

func (u *User) PreSave() {
	if u.ID == "" {
		u.ID = NewID()
	}
	now := GetMillis()
	if u.CreateAt == 0 {
		u.CreateAt = now
	}
	u.UpdateAt = now
	if u.Roles == "" {
		u.Roles = "system_user"
	}
	if u.ActorType == "" {
		u.ActorType = ActorTypeUser
	}
	u.Username = strings.ToLower(u.Username)
	u.Email = strings.ToLower(u.Email)
}

func (u *User) PreUpdate() {
	u.UpdateAt = GetMillis()
}

// IsSystemAdmin reports whether the user's space-separated roles include system_admin.
func (u *User) IsSystemAdmin() bool {
	for _, role := range strings.Fields(u.Roles) {
		if role == "system_admin" {
			return true
		}
	}
	return false
}

func (u *User) Sanitize() {
	u.Email = ""
}
