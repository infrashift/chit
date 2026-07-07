package model

import (
	"strings"
	"testing"
	"time"
)

func validUser() *User {
	return &User{
		ID:          NewID(),
		KratosID:    "kratos-123",
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Roles:       "system_user",
		ActorType:   ActorTypeUser,
		CreateAt:    GetMillis(),
		UpdateAt:    GetMillis(),
	}
}

func TestUser_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*User)
		wantErr bool
	}{
		{"valid user", func(u *User) {}, false},
		{"invalid ID", func(u *User) { u.ID = "bad" }, true},
		{"empty KratosID", func(u *User) { u.KratosID = "" }, true},
		{"username too short (1 char)", func(u *User) { u.Username = "a" }, true},
		{"username too long", func(u *User) { u.Username = strings.Repeat("a", 65) }, true},
		{"username uppercase", func(u *User) { u.Username = "TestUser" }, true},
		{"username leading dash", func(u *User) { u.Username = "-user" }, true},
		{"valid 2-char username", func(u *User) { u.Username = "ab" }, false},
		{"display_name empty", func(u *User) { u.DisplayName = "" }, true},
		{"display_name 100 chars", func(u *User) { u.DisplayName = strings.Repeat("x", 100) }, false},
		{"display_name 101 chars", func(u *User) { u.DisplayName = strings.Repeat("x", 101) }, true},
		{"email empty", func(u *User) { u.Email = "" }, true},
		{"email no @", func(u *User) { u.Email = "nope" }, true},
		{"email 129 chars", func(u *User) { u.Email = strings.Repeat("x", 124) + "@b.co" }, true},
		{"valid email", func(u *User) { u.Email = "a@b.co" }, false},
		{"create_at 0", func(u *User) { u.CreateAt = 0 }, true},
		{"update_at 0", func(u *User) { u.UpdateAt = 0 }, true},
		{"valid agent actor", func(u *User) { u.ActorType = ActorTypeAgent }, false},
		{"valid bot actor", func(u *User) { u.ActorType = ActorTypeBot }, false},
		{"invalid actor_type", func(u *User) { u.ActorType = "robot" }, true},
		{"empty actor_type", func(u *User) { u.ActorType = "" }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := validUser()
			tc.modify(u)
			err := u.IsValid()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

func TestUser_PreSave(t *testing.T) {
	t.Run("generates ID when empty", func(t *testing.T) {
		u := &User{Username: "USER", Email: "FOO@BAR.COM"}
		u.PreSave()
		if u.ID == "" {
			t.Fatal("expected ID to be generated")
		}
		if !IsValidID(u.ID) {
			t.Fatal("expected valid UUID")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existing := NewID()
		u := &User{ID: existing}
		u.PreSave()
		if u.ID != existing {
			t.Fatalf("expected ID to be preserved, got %q", u.ID)
		}
	})

	t.Run("sets CreateAt when 0", func(t *testing.T) {
		u := &User{}
		u.PreSave()
		if u.CreateAt == 0 {
			t.Fatal("expected CreateAt to be set")
		}
	})

	t.Run("preserves existing CreateAt", func(t *testing.T) {
		u := &User{CreateAt: 12345}
		u.PreSave()
		if u.CreateAt != 12345 {
			t.Fatalf("expected CreateAt to be preserved, got %d", u.CreateAt)
		}
	})

	t.Run("always sets UpdateAt", func(t *testing.T) {
		u := &User{UpdateAt: 1}
		u.PreSave()
		if u.UpdateAt == 1 {
			t.Fatal("expected UpdateAt to be overwritten")
		}
	})

	t.Run("defaults Roles to system_user", func(t *testing.T) {
		u := &User{}
		u.PreSave()
		if u.Roles != "system_user" {
			t.Fatalf("expected Roles=%q, got %q", "system_user", u.Roles)
		}
	})

	t.Run("preserves existing Roles", func(t *testing.T) {
		u := &User{Roles: "admin"}
		u.PreSave()
		if u.Roles != "admin" {
			t.Fatalf("expected Roles=%q, got %q", "admin", u.Roles)
		}
	})

	t.Run("lowercases username and email", func(t *testing.T) {
		u := &User{Username: "MyUser", Email: "FOO@BAR.COM"}
		u.PreSave()
		if u.Username != "myuser" {
			t.Fatalf("expected lowercase username, got %q", u.Username)
		}
		if u.Email != "foo@bar.com" {
			t.Fatalf("expected lowercase email, got %q", u.Email)
		}
	})
}

func TestUser_PreUpdate(t *testing.T) {
	u := &User{UpdateAt: 1}
	u.PreUpdate()
	now := time.Now().UnixMilli()
	if u.UpdateAt < now-1000 || u.UpdateAt > now+1000 {
		t.Fatalf("expected UpdateAt near now, got %d", u.UpdateAt)
	}
}

func TestUser_Sanitize(t *testing.T) {
	u := validUser()
	original := u.Username
	u.Sanitize()
	if u.Email != "" {
		t.Fatalf("expected Email to be cleared, got %q", u.Email)
	}
	if u.Username != original {
		t.Fatal("Sanitize should not modify Username")
	}
}
