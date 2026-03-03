package model

import (
	"strings"
	"testing"
	"time"
)

func validPost() *Post {
	return &Post{
		ID:        NewID(),
		ChannelID: NewID(),
		UserID:    NewID(),
		Content:   "Hello world",
		CreateAt:  GetMillis(),
		UpdateAt:  GetMillis(),
	}
}

func TestPost_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Post)
		wantErr bool
	}{
		{"valid post", func(p *Post) {}, false},
		{"valid reply", func(p *Post) { p.RootID = NewID() }, false},
		{"invalid root_id", func(p *Post) { p.RootID = "bad" }, true},
		{"empty content", func(p *Post) { p.Content = "" }, true},
		{"content max size", func(p *Post) { p.Content = strings.Repeat("x", PostMaxContentSize) }, false},
		{"content exceeds max", func(p *Post) { p.Content = strings.Repeat("x", PostMaxContentSize+1) }, true},
		{"invalid channel_id", func(p *Post) { p.ChannelID = "bad" }, true},
		{"invalid user_id", func(p *Post) { p.UserID = "bad" }, true},
		{"invalid post id", func(p *Post) { p.ID = "bad" }, true},
		{"create_at 0", func(p *Post) { p.CreateAt = 0 }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPost()
			tc.modify(p)
			err := p.IsValid()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

func TestPost_PreSave(t *testing.T) {
	p := &Post{}
	p.PreSave()

	if p.ID == "" || !IsValidID(p.ID) {
		t.Fatal("expected valid ID")
	}
	if p.CreateAt == 0 {
		t.Fatal("expected CreateAt to be set")
	}
	if p.UpdateAt == 0 {
		t.Fatal("expected UpdateAt to be set")
	}
	if p.Props == nil {
		t.Fatal("expected Props to be initialized")
	}
}

func TestPost_PreSave_PreservesExisting(t *testing.T) {
	id := NewID()
	p := &Post{ID: id, CreateAt: 999}
	p.PreSave()

	if p.ID != id {
		t.Fatal("expected ID to be preserved")
	}
	if p.CreateAt != 999 {
		t.Fatal("expected CreateAt to be preserved")
	}
}

func TestPost_PreUpdate(t *testing.T) {
	p := &Post{UpdateAt: 1}
	p.PreUpdate()
	now := time.Now().UnixMilli()
	if p.UpdateAt < now-1000 || p.UpdateAt > now+1000 {
		t.Fatalf("expected UpdateAt near now, got %d", p.UpdateAt)
	}
}
