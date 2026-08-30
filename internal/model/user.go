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
	// OAuthClientID links a machine actor to its Ory Hydra OAuth2 client.
	// Empty for humans; stored as NULL rather than "" so the unique index
	// admits any number of users without a client.
	OAuthClientID string `json:"oauth_client_id,omitempty"`
	CreateAt      int64  `json:"create_at"`
	UpdateAt      int64  `json:"update_at"`
	DeleteAt      int64  `json:"delete_at"`
}

func (u *User) IsValid() *AppError {
	if !IsValidID(u.ID) {
		return NewAppError("User.IsValid", "invalid user id", "", http.StatusBadRequest)
	}
	// A user is identified by a PERSON'S Kratos identity or by a MACHINE'S
	// OAuth2 client, and needs at least one. Requiring kratos_id of everything
	// made the machine half of this model unreachable: OAuthClientID,
	// GetByOAuthClientID, ResolveOAuthClient and the oauth cache key all
	// existed and no code path could produce a row for them to find, because
	// the only way to satisfy this check was to give a machine a Kratos
	// identity it does not have and should not be handed.
	if u.KratosID == "" && u.OAuthClientID == "" {
		return NewAppError("User.IsValid",
			"a user needs either a kratos_id (a person) or an oauth_client_id (a machine)",
			"", http.StatusBadRequest)
	}
	if !validUsernameRe.MatchString(u.Username) {
		return NewAppError("User.IsValid", "invalid username", "", http.StatusBadRequest)
	}
	if len(u.DisplayName) == 0 || len(u.DisplayName) > 100 {
		return NewAppError("User.IsValid", "display_name must be 1–100 characters", "", http.StatusBadRequest)
	}
	// EMAIL BELONGS TO A PERSON. It is required for a Kratos-backed user, where
	// it comes from the identity and is how the notifier addresses somebody. A
	// machine actor has no mailbox, and inventing one to pass a validator is
	// the same move as inventing a Kratos identity for it.
	if u.KratosID != "" {
		if len(u.Email) == 0 || len(u.Email) > 128 || !strings.Contains(u.Email, "@") {
			return NewAppError("User.IsValid", "invalid email", "", http.StatusBadRequest)
		}
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
	// The OAuth2 client binding is a credential identifier and is of no use to
	// API consumers; only the provisioning path (which returns the unsanitized
	// saved user) needs to see it.
	u.OAuthClientID = ""
}
