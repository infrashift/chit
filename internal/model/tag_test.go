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

func TestTagNameKey_NFKCAndCaseFold(t *testing.T) {
	// Fullwidth letters (NFKC), upper case, and a letter that folds to two
	// (ß -> ss) all share a key.
	for _, name := range []string{"Strasse", "STRASSE", "Straße", "Ｓｔｒａｓｓｅ"} {
		if got := TagNameKey(name); got != "strasse" {
			t.Errorf("TagNameKey(%q) = %q, want strasse", name, got)
		}
	}
}

func TestValidateTagName(t *testing.T) {
	valid := []string{"a", "bug fix", "v1.2", "snake_case", "kebab-case", "Résumé", "日本語", strings.Repeat("é", MaxTagLength),
		"profile/coder", "Profile/Coder"}
	for _, name := range valid {
		if err := ValidateTagName(name); err != nil {
			t.Errorf("ValidateTagName(%q) = %v, want valid", name, err)
		}
	}
	invalid := []string{"", "   ", strings.Repeat("x", MaxTagLength+1), "a/b", "#tag", "tag!", "profile/", "profile/a/b", "emoji 🙂"}
	for _, name := range invalid {
		if err := ValidateTagName(name); err == nil {
			t.Errorf("ValidateTagName(%q) = nil, want an error", name)
		}
	}
}

func TestIsReservedTagName(t *testing.T) {
	for name, want := range map[string]bool{
		"profile/coder": true, "PROFILE/coder": true, "ｐｒｏｆｉｌｅ/coder": true,
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
