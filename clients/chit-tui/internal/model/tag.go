package model

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// Tag scopes, as the server names them.
const (
	TagScopePrivate = "private"
	TagScopeChannel = "channel"
)

// Tag limits, as the server applies them.
const (
	// MaxTagLength is the most runes in a tag name.
	MaxTagLength = 50
	// ReservedTagPrefix starts the names that only machines may make.
	ReservedTagPrefix = "profile/"
	// MaxSearchTags is the most tags one search or channel find accepts.
	MaxSearchTags = 8
)

// Tag mirrors the server's Tag model. A channel tag belongs to ChannelID and
// is shared by its members; a private tag is seen by its owner only.
type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope,omitempty"`
	OwnerID   string `json:"owner_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	CreateAt  int64  `json:"create_at,omitempty"`
	UpdateAt  int64  `json:"update_at,omitempty"`
}

// TaggedChannel is one result of a find by tags: a channel and the channel
// tags that the caller can see on it.
type TaggedChannel struct {
	Channel *Channel `json:"channel"`
	Tags    []*Tag   `json:"tags"`
}

// Private reports whether only its owner sees the tag.
func (t *Tag) Private() bool { return t.Scope == TagScopePrivate }

// UsableIn reports whether the tag can be applied in channelID: a channel
// tag in its own channel, a private tag anywhere its owner can read.
func (t *Tag) UsableIn(channelID string) bool {
	return t.Scope != TagScopeChannel || t.ChannelID == channelID
}

// NormalizeTagName returns the name as the server stores it: NFKC, with the
// spaces at both ends removed.
func NormalizeTagName(name string) string {
	out := strings.TrimSpace(norm.NFKC.String(name))
	assert.That(out == strings.TrimSpace(out), "NormalizeTagName: result not trimmed")
	return out
}

// ValidTagName checks a name that a person types, with the server's rules:
// 1 to MaxTagLength runes, each a letter, a digit, a space, '-', '_' or '.'.
// The reserved prefix is refused, because the server refuses it for a
// person. The error text is for the person.
func ValidTagName(raw string) error {
	name := NormalizeTagName(raw)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > MaxTagLength {
		return errors.New("a tag name must have 1 to 50 characters")
	}
	if strings.HasPrefix(strings.ToLower(name), ReservedTagPrefix) {
		return errors.New("the profile/ prefix is reserved")
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' && r != '_' && r != '.' {
			return errors.New("a tag name can have only letters, digits, spaces, '-', '_' and '.'")
		}
	}
	assert.That(n >= 1 && n <= MaxTagLength, "ValidTagName: accepted a name of a bad length")
	return nil
}
