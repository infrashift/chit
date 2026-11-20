package model

import (
	"net/http"
	"regexp"

	"github.com/infrashift/chit/internal/assert"
)

const (
	ChannelOpen    = "O"
	ChannelPrivate = "P"
	ChannelDirect  = "D"
	ChannelGroup   = "G"
)

// semscope: allow(NASA-P10-06) compiled once, never changed
var validChannelNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

type Channel struct {
	ID            string `json:"id"`
	TeamID        string `json:"team_id,omitempty"`
	CreatorID     string `json:"creator_id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Header        string `json:"header"`
	Purpose       string `json:"purpose"`
	Type          string `json:"type"`
	TotalMsgCount int64  `json:"total_msg_count"`
	LastPostAt    int64  `json:"last_post_at"`
	CreateAt      int64  `json:"create_at"`
	UpdateAt      int64  `json:"update_at"`
	DeleteAt      int64  `json:"delete_at"`
}

func (c *Channel) IsValid() *AppError {
	assert.That(c != nil, "Channel.IsValid: nil channel")
	if !IsValidID(c.ID) {
		return NewAppError("Channel.IsValid", "invalid channel id", "", http.StatusBadRequest)
	}
	if c.Type != ChannelDirect && c.Type != ChannelGroup {
		if !validChannelNameRe.MatchString(c.Name) {
			return NewAppError("Channel.IsValid", "invalid channel name", "", http.StatusBadRequest)
		}
	}
	if c.Type == ChannelDirect || c.Type == ChannelGroup {
		if c.DisplayName == "" || len(c.DisplayName) > 256 {
			return NewAppError("Channel.IsValid", "display_name must be 1–256 characters", "", http.StatusBadRequest)
		}
	} else if c.DisplayName == "" || len(c.DisplayName) > 64 {
		return NewAppError("Channel.IsValid", "display_name must be 1–64 characters", "", http.StatusBadRequest)
	}
	if len(c.Header) > 1024 {
		return NewAppError("Channel.IsValid", "header must be at most 1024 characters", "", http.StatusBadRequest)
	}
	if len(c.Purpose) > 250 {
		return NewAppError("Channel.IsValid", "purpose must be at most 250 characters", "", http.StatusBadRequest)
	}
	switch c.Type {
	case ChannelOpen, ChannelPrivate, ChannelDirect, ChannelGroup:
	default:
		return NewAppError("Channel.IsValid", "invalid channel type", "", http.StatusBadRequest)
	}
	if c.CreateAt == 0 {
		return NewAppError("Channel.IsValid", "create_at is required", "", http.StatusBadRequest)
	}
	return nil
}

func (c *Channel) PreSave() {
	if c.ID == "" {
		c.ID = NewID()
	}
	now := GetMillis()
	if c.CreateAt == 0 {
		c.CreateAt = now
	}
	c.UpdateAt = now
	assert.That(c.ID != "" && c.CreateAt != 0, "Channel.PreSave: channel left without id or create_at")
	assert.That(c.UpdateAt > 0, "Channel.PreSave: update_at not set")
}

func (c *Channel) PreUpdate() {
	assert.That(c != nil, "Channel.PreUpdate: nil channel")
	c.UpdateAt = GetMillis()
	assert.That(c.UpdateAt > 0, "Channel.PreUpdate: update_at not set")
}

type ChannelMember struct {
	ChannelID    string         `json:"channel_id"`
	UserID       string         `json:"user_id"`
	Roles        string         `json:"roles"`
	LastViewedAt int64          `json:"last_viewed_at"`
	MsgCount     int64          `json:"msg_count"`
	MentionCount int64          `json:"mention_count"`
	NotifyProps  map[string]any `json:"notify_props"`
	CreateAt     int64          `json:"create_at"`
}

func (cm *ChannelMember) IsValid() *AppError {
	assert.That(cm != nil, "ChannelMember.IsValid: nil member")
	if !IsValidID(cm.ChannelID) {
		return NewAppError("ChannelMember.IsValid", "invalid channel_id", "", http.StatusBadRequest)
	}
	if !IsValidID(cm.UserID) {
		return NewAppError("ChannelMember.IsValid", "invalid user_id", "", http.StatusBadRequest)
	}
	return nil
}

func (cm *ChannelMember) PreSave() {
	if cm.Roles == "" {
		cm.Roles = "channel_user"
	}
	if cm.CreateAt == 0 {
		cm.CreateAt = GetMillis()
	}
	if cm.NotifyProps == nil {
		cm.NotifyProps = make(map[string]any)
	}
	assert.That(cm.Roles != "" && cm.CreateAt != 0, "ChannelMember.PreSave: roles or create_at not set")
	assert.That(cm.NotifyProps != nil, "ChannelMember.PreSave: nil notify props")
}

// GetPostsOptions defines pagination and filtering options for post queries.
type GetPostsOptions struct {
	Page    int
	PerPage int
	// Since, when set, returns only posts created after it, oldest first.
	Since int64
}
