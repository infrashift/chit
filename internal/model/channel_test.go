package model

import (
	"strings"
	"testing"
	"time"
)

func validChannel() *Channel {
	return &Channel{
		ID:          NewID(),
		TeamID:      NewID(),
		CreatorID:   NewID(),
		Name:        "general",
		DisplayName: "General",
		Type:        ChannelOpen,
		CreateAt:    GetMillis(),
		UpdateAt:    GetMillis(),
	}
}

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Channel)
		wantErr bool
	}{
		{"valid O channel", func(c *Channel) {}, false},
		{"valid D channel skips name regex", func(c *Channel) {
			c.Type = ChannelDirect
			c.Name = "ANY__NAME"
		}, false},
		{"invalid ID", func(c *Channel) { c.ID = "bad" }, true},
		{"bad name for O channel", func(c *Channel) { c.Name = "BAD_NAME" }, true},
		{"display_name empty", func(c *Channel) { c.DisplayName = "" }, true},
		{"display_name 64 chars", func(c *Channel) { c.DisplayName = strings.Repeat("x", 64) }, false},
		{"display_name 65 chars", func(c *Channel) { c.DisplayName = strings.Repeat("x", 65) }, true},
		{"header 1024 chars", func(c *Channel) { c.Header = strings.Repeat("x", 1024) }, false},
		{"header 1025 chars", func(c *Channel) { c.Header = strings.Repeat("x", 1025) }, true},
		{"purpose 250 chars", func(c *Channel) { c.Purpose = strings.Repeat("x", 250) }, false},
		{"purpose 251 chars", func(c *Channel) { c.Purpose = strings.Repeat("x", 251) }, true},
		{"invalid type", func(c *Channel) { c.Type = "Z" }, true},
		{"type P", func(c *Channel) { c.Type = ChannelPrivate }, false},
		{"type G skips name", func(c *Channel) {
			c.Type = ChannelGroup
			c.Name = "ANYTHING"
		}, false},
		{"create_at 0", func(c *Channel) { c.CreateAt = 0 }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := validChannel()
			tc.modify(ch)
			err := ch.IsValid()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

func TestChannel_PreSave(t *testing.T) {
	ch := &Channel{}
	ch.PreSave()

	if ch.ID == "" || !IsValidID(ch.ID) {
		t.Fatal("expected valid ID")
	}
	if ch.CreateAt == 0 {
		t.Fatal("expected CreateAt to be set")
	}
	if ch.UpdateAt == 0 {
		t.Fatal("expected UpdateAt to be set")
	}
}

func TestChannel_PreUpdate(t *testing.T) {
	ch := &Channel{UpdateAt: 1}
	ch.PreUpdate()
	now := time.Now().UnixMilli()
	if ch.UpdateAt < now-1000 || ch.UpdateAt > now+1000 {
		t.Fatalf("expected UpdateAt near now, got %d", ch.UpdateAt)
	}
}

func TestChannelMember_IsValid(t *testing.T) {
	cm := &ChannelMember{ChannelID: NewID(), UserID: NewID()}
	if err := cm.IsValid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	cm.ChannelID = "bad"
	if err := cm.IsValid(); err == nil {
		t.Fatal("expected error for invalid ChannelID")
	}

	cm.ChannelID = NewID()
	cm.UserID = "bad"
	if err := cm.IsValid(); err == nil {
		t.Fatal("expected error for invalid UserID")
	}
}

func TestChannelMember_PreSave(t *testing.T) {
	cm := &ChannelMember{}
	cm.PreSave()

	if cm.Roles != "channel_user" {
		t.Fatalf("expected Roles=%q, got %q", "channel_user", cm.Roles)
	}
	if cm.CreateAt == 0 {
		t.Fatal("expected CreateAt to be set")
	}
	if cm.NotifyProps == nil {
		t.Fatal("expected NotifyProps to be initialized")
	}

	// Existing NotifyProps preserved
	existing := map[string]any{"desktop": "all"}
	cm2 := &ChannelMember{NotifyProps: existing}
	cm2.PreSave()
	if cm2.NotifyProps["desktop"] != "all" {
		t.Fatal("expected existing NotifyProps to be preserved")
	}
}
