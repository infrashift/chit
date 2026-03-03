package model

import (
	"strings"
	"testing"
)

func TestTag_IsValid(t *testing.T) {
	tag := &Tag{ID: NewID(), Name: "important"}
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
	if err := tag.IsValid(); err == nil {
		t.Fatal("expected error for name > 50 chars")
	}

	tag.Name = strings.Repeat("x", 50)
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
