package mention

import (
	"testing"
)

func TestExtractMentionPrefix_Active(t *testing.T) {
	prefix, startCol, active := ExtractMentionPrefix("@us", 3)
	if !active {
		t.Fatal("expected active")
	}
	if prefix != "us" {
		t.Errorf("prefix = %q, want %q", prefix, "us")
	}
	if startCol != 0 {
		t.Errorf("startCol = %d, want 0", startCol)
	}
}

func TestExtractMentionPrefix_MidLine(t *testing.T) {
	prefix, startCol, active := ExtractMentionPrefix("hello @bo", 9)
	if !active {
		t.Fatal("expected active")
	}
	if prefix != "bo" {
		t.Errorf("prefix = %q, want %q", prefix, "bo")
	}
	if startCol != 6 {
		t.Errorf("startCol = %d, want 6", startCol)
	}
}

func TestExtractMentionPrefix_NoAt(t *testing.T) {
	_, _, active := ExtractMentionPrefix("hello", 5)
	if active {
		t.Error("expected inactive without @")
	}
}

func TestExtractMentionPrefix_SpaceAfterAt(t *testing.T) {
	_, _, active := ExtractMentionPrefix("@ us", 4)
	if active {
		t.Error("expected inactive with space after @")
	}
}

func TestExtractMentionPrefix_JustAt(t *testing.T) {
	prefix, startCol, active := ExtractMentionPrefix("@", 1)
	if !active {
		t.Fatal("expected active for bare @")
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
	if startCol != 0 {
		t.Errorf("startCol = %d, want 0", startCol)
	}
}

func TestFilterEntries_PrefixMatch(t *testing.T) {
	entries := []MentionEntry{
		{Username: "alice", DisplayName: "Alice A"},
		{Username: "bob", DisplayName: "Bob B"},
		{Username: "all", Special: true},
	}
	filtered := FilterEntries(entries, "al")
	if len(filtered) != 2 {
		t.Fatalf("got %d, want 2", len(filtered))
	}
	// Special first
	if filtered[0].Username != "all" {
		t.Errorf("first = %q, want special 'all'", filtered[0].Username)
	}
	if filtered[1].Username != "alice" {
		t.Errorf("second = %q, want 'alice'", filtered[1].Username)
	}
}

func TestFilterEntries_DisplayNameMatch(t *testing.T) {
	entries := []MentionEntry{
		{Username: "jdoe", DisplayName: "John Doe"},
	}
	filtered := FilterEntries(entries, "John")
	if len(filtered) != 1 {
		t.Fatalf("got %d, want 1", len(filtered))
	}
}

func TestFilterEntries_EmptyPrefix(t *testing.T) {
	entries := []MentionEntry{
		{Username: "alice"},
		{Username: "bob"},
		{Username: "all", Special: true},
	}
	filtered := FilterEntries(entries, "")
	if len(filtered) != 3 {
		t.Fatalf("got %d, want 3", len(filtered))
	}
	// Special first
	if filtered[0].Username != "all" {
		t.Errorf("first = %q, want 'all'", filtered[0].Username)
	}
}

func TestFilterEntries_CaseInsensitive(t *testing.T) {
	entries := []MentionEntry{
		{Username: "Alice"},
	}
	filtered := FilterEntries(entries, "aL")
	if len(filtered) != 1 {
		t.Fatalf("got %d, want 1", len(filtered))
	}
}

func TestFindMentions_Single(t *testing.T) {
	spans := FindMentions("hello @alice how are you")
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Username != "alice" {
		t.Errorf("username = %q", spans[0].Username)
	}
	if spans[0].Start != 6 || spans[0].End != 12 {
		t.Errorf("span = [%d, %d), want [6, 12)", spans[0].Start, spans[0].End)
	}
}

func TestFindMentions_Multiple(t *testing.T) {
	spans := FindMentions("@alice @bob")
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if spans[0].Username != "alice" {
		t.Errorf("first = %q", spans[0].Username)
	}
	if spans[1].Username != "bob" {
		t.Errorf("second = %q", spans[1].Username)
	}
}

func TestFindMentions_Special(t *testing.T) {
	spans := FindMentions("@all please review")
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Username != "all" {
		t.Errorf("username = %q", spans[0].Username)
	}
}

func TestFindMentions_None(t *testing.T) {
	spans := FindMentions("no mentions here")
	if len(spans) != 0 {
		t.Fatalf("got %d spans, want 0", len(spans))
	}
}

// The cursor column is in characters, but the text was indexed by byte, so
// an accented letter before the @ shifted everything after it.
func TestExtractMentionPrefix_CountsCharactersNotBytes(t *testing.T) {
	prefix, startCol, active := ExtractMentionPrefix("héllo @bo", 9)
	if !active || prefix != "bo" || startCol != 6 {
		t.Errorf("got (%q, %d, %v), want (\"bo\", 6, true)", prefix, startCol, active)
	}
}

// An @ inside a word is an email address, not a mention.
func TestExtractMentionPrefix_IgnoresEmailAddresses(t *testing.T) {
	if _, _, active := ExtractMentionPrefix("mail foo@b", 10); active {
		t.Error("an email address opened the mention popup")
	}
	if _, _, active := ExtractMentionPrefix("(@b", 3); !active {
		t.Error("a mention after punctuation was not recognized")
	}
}

// The trailing full stop of a sentence was taken as part of the name, so
// "@bob." did not highlight as a mention of bob.
func TestFindMentions_SentencePunctuationIsNotPartOfTheName(t *testing.T) {
	spans := FindMentions("thanks @bob.")
	if len(spans) != 1 || spans[0].Username != "bob" {
		t.Errorf("spans = %+v, want one mention of bob", spans)
	}
}

func TestFindMentions_IgnoresEmailAddresses(t *testing.T) {
	if spans := FindMentions("write to someone@example.com"); len(spans) != 0 {
		t.Errorf("spans = %+v, want none", spans)
	}
}
