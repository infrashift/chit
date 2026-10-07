package model

import (
	"strings"
	"testing"
	"time"
)

func validTeam() *Team {
	return &Team{
		ID:          NewID(),
		Name:        "engineering",
		DisplayName: "Engineering",
		Description: "The engineering team",
		Type:        TeamOpen,
		CreatorID:   NewID(),
		CreateAt:    GetMillis(),
		UpdateAt:    GetMillis(),
	}
}

func TestTeam_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Team)
		wantErr bool
	}{
		{"valid team", func(t *Team) {}, false},
		{"invalid ID", func(t *Team) { t.ID = "bad" }, true},
		{"name uppercase", func(t *Team) { t.Name = "Engineering" }, true},
		{"name underscore", func(t *Team) { t.Name = "eng_team" }, true},
		{"name leading dash", func(t *Team) { t.Name = "-team" }, true},
		{"valid name", func(t *Team) { t.Name = "eng-team" }, false},
		{"display_name empty", func(t *Team) { t.DisplayName = "" }, true},
		{"display_name 64 chars", func(t *Team) { t.DisplayName = strings.Repeat("x", 64) }, false},
		{"display_name 65 chars", func(t *Team) { t.DisplayName = strings.Repeat("x", 65) }, true},
		{"description 255 chars", func(t *Team) { t.Description = strings.Repeat("x", 255) }, false},
		{"description 256 chars", func(t *Team) { t.Description = strings.Repeat("x", 256) }, true},
		{"invalid type", func(t *Team) { t.Type = "X" }, true},
		{"type invite only", func(t *Team) { t.Type = TeamInviteOnly }, false},
		{"create_at 0", func(t *Team) { t.CreateAt = 0 }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := validTeam()
			tc.modify(tm)
			err := tm.IsValid()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

func TestTeam_PreSave(t *testing.T) {
	tm := &Team{}
	tm.PreSave()

	if tm.ID == "" || !IsValidID(tm.ID) {
		t.Fatal("expected valid ID")
	}
	if tm.CreateAt == 0 {
		t.Fatal("expected CreateAt to be set")
	}
	if tm.UpdateAt == 0 {
		t.Fatal("expected UpdateAt to be set")
	}
	if tm.Type != TeamOpen {
		t.Fatalf("expected default Type=%q, got %q", TeamOpen, tm.Type)
	}
}

func TestTeam_PreSave_PreservesExisting(t *testing.T) {
	id := NewID()
	tm := &Team{ID: id, CreateAt: 999, Type: TeamInviteOnly}
	tm.PreSave()

	if tm.ID != id {
		t.Fatal("expected ID to be preserved")
	}
	if tm.CreateAt != 999 {
		t.Fatal("expected CreateAt to be preserved")
	}
	if tm.Type != TeamInviteOnly {
		t.Fatal("expected Type to be preserved")
	}
}

func TestTeam_PreUpdate(t *testing.T) {
	tm := &Team{UpdateAt: 1}
	tm.PreUpdate()
	now := time.Now().UnixMilli()
	if tm.UpdateAt < now-1000 || tm.UpdateAt > now+1000 {
		t.Fatalf("expected UpdateAt near now, got %d", tm.UpdateAt)
	}
}

func TestTeamMember_IsValid(t *testing.T) {
	tm := &TeamMember{TeamID: NewID(), UserID: NewID()}
	if err := tm.IsValid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	tm.TeamID = "bad"
	if err := tm.IsValid(); err == nil {
		t.Fatal("expected error for invalid TeamID")
	}

	tm.TeamID = NewID()
	tm.UserID = "bad"
	if err := tm.IsValid(); err == nil {
		t.Fatal("expected error for invalid UserID")
	}
}

func TestTeamMember_PreSave(t *testing.T) {
	tm := &TeamMember{}
	tm.PreSave()

	if tm.Roles != "team_user" {
		t.Fatalf("expected Roles=%q, got %q", "team_user", tm.Roles)
	}
	if tm.CreateAt == 0 {
		t.Fatal("expected CreateAt to be set")
	}

	// Existing roles preserved
	tm2 := &TeamMember{Roles: "team_admin"}
	tm2.PreSave()
	if tm2.Roles != "team_admin" {
		t.Fatal("expected existing Roles to be preserved")
	}
}

func TestTeamMember_IsTeamAdmin(t *testing.T) {
	cases := map[string]bool{
		"team_admin":           true,
		"team_user team_admin": true,
		"team_user":            false,
		"":                     false,
		"team_administrator":   false,
	}
	for roles, want := range cases {
		if got := (&TeamMember{Roles: roles}).IsTeamAdmin(); got != want {
			t.Errorf("IsTeamAdmin(%q) = %v, want %v", roles, got, want)
		}
	}
}
