package model

import (
	"strings"
	"testing"
)

func TestTag_IsValid(t *testing.T) {
	tag := &Tag{ID: NewID(), Name: "important", Scope: TagScopePrivate, OwnerID: NewID()}
	tag.NameKey = TagNameKey(tag.Name)
	if err := tag.IsValid(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	tag.ID = "bad"
	if err := tag.IsValid(); err == nil {
		t.Fatal("expected error for invalid ID")
	}

	tag.ID = NewID()
	tag.Name = ""
	if err := tag.IsValid(); err == nil {
		t.Fatal("expected error for empty name")
	}

	tag.Name = strings.Repeat("x", 51)
	tag.NameKey = TagNameKey(tag.Name)
	if err := tag.IsValid(); err == nil {
		t.Fatal("expected error for name > 50 chars")
	}

	tag.Name = strings.Repeat("x", 50)
	tag.NameKey = TagNameKey(tag.Name)
	if err := tag.IsValid(); err != nil {
		t.Fatalf("expected 50-char name to be valid, got %v", err)
	}
}

func TestTag_PreSave(t *testing.T) {
	tag := &Tag{}
	tag.PreSave()
	if tag.ID == "" || !IsValidID(tag.ID) {
		t.Fatal("expected ID to be generated")
	}

	existing := NewID()
	tag2 := &Tag{ID: existing}
	tag2.PreSave()
	if tag2.ID != existing {
		t.Fatal("expected ID to be preserved")
	}
}

func TestTagNameKey_NFKCAndLowercase(t *testing.T) {
	for name, want := range map[string]string{
		"Strasse": "strasse", "STRASSE": "strasse", "Ｓｔｒａｓｓｅ": "strasse",
		// Simple lowercasing, not folding: ß stays ß, so "Straße" and
		// "Strasse" are two names.
		"Straße": "straße",
		"Résumé": "résumé", "RE\u0301SUME\u0301": "résumé",
	} {
		if got := TagNameKey(name); got != want {
			t.Errorf("TagNameKey(%q) = %q, want %q", name, got, want)
		}
	}
	// Spaces around a name are trimmed before the key is made.
	if got := TagNameKey("\tSTRASSE "); got != "strasse" {
		t.Errorf("TagNameKey with spaces = %q, want strasse", got)
	}
}

func TestValidateTagName(t *testing.T) {
	valid := []string{"a", "bug fix", "v1.2", "snake_case", "kebab-case", "Résumé", "日本語", strings.Repeat("é", MaxTagLength),
		"profile/coder", "Profile/Coder",
		// U+0130 (İ) lowercases to the one-byte "i", so the prefix is shorter
		// in the key than in the name (ISSUE-012).
		"profİle/coder", "PROFİLE/İstanbul"}
	for _, name := range valid {
		if err := ValidateTagName(name); err != nil {
			t.Errorf("ValidateTagName(%q) = %v, want valid", name, err)
		}
	}
	invalid := []string{"", "   ", strings.Repeat("x", MaxTagLength+1), "a/b", "#tag", "tag!", "profile/", "profile/a/b", "emoji 🙂",
		"profİle/", "profİle/a/b"}
	for _, name := range invalid {
		if err := ValidateTagName(name); err == nil {
			t.Errorf("ValidateTagName(%q) = nil, want an error", name)
		}
	}
}

func TestIsReservedTagName(t *testing.T) {
	for name, want := range map[string]bool{
		"profile/coder": true, "PROFILE/coder": true, "ｐｒｏｆｉｌｅ/coder": true, "profİle/coder": true,
		"profiles": false, "my profile/x": false, "profile": false,
	} {
		if got := IsReservedTagName(name); got != want {
			t.Errorf("IsReservedTagName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTag_VisibleTo(t *testing.T) {
	owner, other := NewID(), NewID()
	private := &Tag{Scope: TagScopePrivate, OwnerID: owner}
	channel := &Tag{Scope: TagScopeChannel, OwnerID: owner}
	if !private.VisibleTo(owner) || private.VisibleTo(other) {
		t.Error("a private tag must be visible to its owner only")
	}
	if !channel.VisibleTo(other) {
		t.Error("a channel tag is visible to every reader of the item")
	}
}
