package model

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Tag scopes. A private Tag is seen only by its owner, on any channel or post
// the owner can read. A channel Tag belongs to one channel: every member of
// that channel sees it and may apply it there, and only there.
const (
	TagScopePrivate = "private"
	TagScopeChannel = "channel"
)

// Tag limits (HALO PLAN section 3.12).
const (
	// MaxTagLength is the most characters (runes) a Tag name has.
	MaxTagLength = 50
	// MaxTagsPerItem bounds the Tags on one channel or post, counted per
	// reader's view: the channel Tags, and each owner's private Tags, are
	// limited separately, so the limit never reveals another owner's
	// private Tags.
	MaxTagsPerItem = 32
	// MaxTagsPerOwner is the most Tags one actor owns.
	MaxTagsPerOwner = 1024
	// MaxTagFilter is the most Tags one channel filter or search names.
	MaxTagFilter = 8
)

// ReservedTagPrefix starts the Profile Tags that HALO's halod applies on a
// person's behalf. A person cannot create, rename or apply such a Tag; only
// an allowlisted client acting for the owner (X-On-Behalf-Of) can, and only
// as the owner's private Tag.
const ReservedTagPrefix = "profile/"

// SystemActorID owns the Tags that migration 000008 converted from the old
// global vocabulary, whose creators Chit never recorded. It is the user row
// "chit-system" that the migration adds.
const SystemActorID = "00000000-0000-7000-8000-000000000001"

type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	OwnerID   string `json:"owner_id"`
	ChannelID string `json:"channel_id,omitempty"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	// NameKey is the name after NFKC normalization and case folding; a name
	// is unique per owner (private) or per channel (channel) by its key.
	NameKey string `json:"-"`
}

// VisibleTo reports whether userID may see the Tag on an item userID can
// read. A channel Tag can only be on items of its own channel, so a reader
// of the item is a member of that channel. A private Tag is its owner's
// alone.
func (t *Tag) VisibleTo(userID string) bool {
	return t.Scope == TagScopeChannel || t.OwnerID == userID
}

// IsReserved reports whether the Tag's name starts with ReservedTagPrefix.
func (t *Tag) IsReserved() bool {
	return IsReservedTagName(t.Name)
}

func (t *Tag) IsValid() *AppError {
	if !IsValidID(t.ID) {
		return NewAppError("Tag.IsValid", "invalid tag id", "", http.StatusBadRequest)
	}
	if !IsValidID(t.OwnerID) {
		return NewAppError("Tag.IsValid", "invalid owner_id", "", http.StatusBadRequest)
	}
	switch t.Scope {
	case TagScopePrivate:
		if t.ChannelID != "" {
			return NewBadRequestError("Tag.IsValid", "a private tag names no channel")
		}
	case TagScopeChannel:
		if !IsValidID(t.ChannelID) {
			return NewBadRequestError("Tag.IsValid", "a channel tag needs a channel_id")
		}
	default:
		return NewBadRequestError("Tag.IsValid", "scope must be private or channel")
	}
	if err := ValidateTagName(t.Name); err != nil {
		return err
	}
	if t.NameKey != TagNameKey(t.Name) {
		return NewBadRequestError("Tag.IsValid", "name key does not match the name")
	}
	return nil
}

// PreSave sets the ID, the normalized name and its key, and the times.
func (t *Tag) PreSave() {
	if t.ID == "" {
		t.ID = NewID()
	}
	t.Name = NormalizeTagName(t.Name)
	t.NameKey = TagNameKey(t.Name)
	now := GetMillis()
	if t.CreateAt == 0 {
		t.CreateAt = now
	}
	t.UpdateAt = now
}

// NormalizeTagName returns the name as stored: NFKC-normalized, with spaces
// trimmed from both ends. Case is kept for display.
func NormalizeTagName(name string) string {
	return strings.TrimSpace(norm.NFKC.String(name))
}

// TagNameKey is the key a name is unique by: NFKC, then case folding.
// "Résumé", "RÉSUMÉ" and "ｒésumé" share one key.
func TagNameKey(name string) string {
	return cases.Fold().String(NormalizeTagName(name))
}

// IsReservedTagName reports whether name starts with the reserved prefix,
// whatever its case or width.
func IsReservedTagName(name string) bool {
	return strings.HasPrefix(TagNameKey(name), ReservedTagPrefix)
}

// ValidateTagName checks a name after NormalizeTagName: 1 to MaxTagLength
// characters, each a letter, a digit, a space, '-', '_' or '.'. The one
// exception is the reserved prefix "profile/": its slash is allowed there and
// nowhere else, and the rest of the name follows the same rule.
func ValidateTagName(raw string) *AppError {
	name := NormalizeTagName(raw)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > MaxTagLength {
		return NewBadRequestError("Tag.IsValid", "name must be 1 to 50 characters")
	}
	rest := name
	if IsReservedTagName(name) {
		// The prefix is ASCII in the folded key; in the name it may differ
		// in case only, so it has the same byte length.
		rest = name[len(ReservedTagPrefix):]
		if rest == "" {
			return NewBadRequestError("Tag.IsValid", "a profile/ tag needs a name after the prefix")
		}
	}
	for _, r := range rest {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' && r != '_' && r != '.' {
			return NewBadRequestError("Tag.IsValid", "name may hold only letters, digits, spaces, '-', '_' and '.'")
		}
	}
	return nil
}

// MessageTag is a Tag applied to a post (message_tags).
type MessageTag struct {
	MessageID string `json:"message_id"`
	TagID     string `json:"tag_id"`
	AppliedBy string `json:"applied_by,omitempty"`
}

// ChannelTag is a Tag applied to a channel (channel_tags).
type ChannelTag struct {
	ChannelID string `json:"channel_id"`
	TagID     string `json:"tag_id"`
	AppliedBy string `json:"applied_by,omitempty"`
}

// TaggedChannel is a channel in a tag-filtered channel list, with the
// channel-level Tags that the reader can see.
type TaggedChannel struct {
	Channel *Channel `json:"channel"`
	Tags    []*Tag   `json:"tags"`
}
