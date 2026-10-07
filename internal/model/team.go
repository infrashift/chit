package model

import (
	"net/http"
	"regexp"
)

const (
	TeamOpen       = "O"
	TeamInviteOnly = "I"
)

var validTeamNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

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

func (t *Team) IsValid() *AppError {
	if !IsValidID(t.ID) {
		return NewAppError("Team.IsValid", "invalid team id", "", http.StatusBadRequest)
	}
	if !validTeamNameRe.MatchString(t.Name) {
		return NewAppError("Team.IsValid", "invalid team name", "", http.StatusBadRequest)
	}
	if t.DisplayName == "" || len(t.DisplayName) > 64 {
		return NewAppError("Team.IsValid", "display_name must be 1–64 characters", "", http.StatusBadRequest)
	}
	if len(t.Description) > 255 {
		return NewAppError("Team.IsValid", "description must be at most 255 characters", "", http.StatusBadRequest)
	}
	if t.Type != TeamOpen && t.Type != TeamInviteOnly {
		return NewAppError("Team.IsValid", "invalid team type", "", http.StatusBadRequest)
	}
	if t.CreateAt == 0 {
		return NewAppError("Team.IsValid", "create_at is required", "", http.StatusBadRequest)
	}
	return nil
}

func (t *Team) PreSave() {
	if t.ID == "" {
		t.ID = NewID()
	}
	now := GetMillis()
	if t.CreateAt == 0 {
		t.CreateAt = now
	}
	t.UpdateAt = now
	if t.Type == "" {
		t.Type = TeamOpen
	}
}

func (t *Team) PreUpdate() {
	t.UpdateAt = GetMillis()
}

type TeamMember struct {
	TeamID   string `json:"team_id"`
	UserID   string `json:"user_id"`
	Roles    string `json:"roles"`
	CreateAt int64  `json:"create_at"`
	DeleteAt int64  `json:"delete_at"`
}

func (tm *TeamMember) IsValid() *AppError {
	if !IsValidID(tm.TeamID) {
		return NewAppError("TeamMember.IsValid", "invalid team_id", "", http.StatusBadRequest)
	}
	if !IsValidID(tm.UserID) {
		return NewAppError("TeamMember.IsValid", "invalid user_id", "", http.StatusBadRequest)
	}
	return nil
}

func (tm *TeamMember) PreSave() {
	if tm.Roles == "" {
		tm.Roles = "team_user"
	}
	if tm.CreateAt == 0 {
		tm.CreateAt = GetMillis()
	}
}
