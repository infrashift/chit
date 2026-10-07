package tagpicker_test

import (
	"reflect"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
)

func TestExtractHashtags_Basic(t *testing.T) {
	tags := tagpicker.ExtractHashtags("hello #urgent world")
	if !reflect.DeepEqual(tags, []string{"urgent"}) {
		t.Errorf("got %v", tags)
	}
}

func TestExtractHashtags_Multiple(t *testing.T) {
	tags := tagpicker.ExtractHashtags("fix #bug #urgent please")
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tags)
	}
}

func TestExtractHashtags_NoTags(t *testing.T) {
	tags := tagpicker.ExtractHashtags("hello world")
	if tags != nil {
		t.Errorf("expected nil, got %v", tags)
	}
}

func TestExtractHashtags_Dedup(t *testing.T) {
	tags := tagpicker.ExtractHashtags("#urgent #Urgent")
	if !reflect.DeepEqual(tags, []string{"urgent"}) {
		t.Errorf("expected dedup, got %v", tags)
	}
}

func TestExtractHashtags_StartOfString(t *testing.T) {
	tags := tagpicker.ExtractHashtags("#urgent fix now")
	if !reflect.DeepEqual(tags, []string{"urgent"}) {
		t.Errorf("got %v", tags)
	}
}

func TestExtractHashtags_WithDashUnderscore(t *testing.T) {
	tags := tagpicker.ExtractHashtags("use #my-tag and #my_tag2")
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", tags)
	}
}

func TestStripHashtags_Basic(t *testing.T) {
	cleaned, tags := tagpicker.StripHashtags("hello #urgent world")
	if cleaned != "hello world" {
		t.Errorf("cleaned = %q", cleaned)
	}
	if !reflect.DeepEqual(tags, []string{"urgent"}) {
		t.Errorf("tags = %v", tags)
	}
}

func TestStripHashtags_NoTags(t *testing.T) {
	cleaned, tags := tagpicker.StripHashtags("hello world")
	if cleaned != "hello world" {
		t.Errorf("cleaned = %q", cleaned)
	}
	if tags != nil {
		t.Errorf("tags = %v", tags)
	}
}

func TestStripHashtags_Multiple(t *testing.T) {
	cleaned, tags := tagpicker.StripHashtags("fix #bug #urgent please")
	if cleaned != "fix please" {
		t.Errorf("cleaned = %q", cleaned)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %v", tags)
	}
}

func TestStripHashtags_TrailingTag(t *testing.T) {
	cleaned, tags := tagpicker.StripHashtags("hello #urgent")
	if cleaned != "hello" {
		t.Errorf("cleaned = %q", cleaned)
	}
	if !reflect.DeepEqual(tags, []string{"urgent"}) {
		t.Errorf("tags = %v", tags)
	}
}

// Stripping tags used to collapse every run of spaces in the message and
// eat the newline before a tag, flattening indented code and line breaks.
func TestStripHashtags_LeavesTheRestOfTheMessageAlone(t *testing.T) {
	tests := []struct {
		name, in, want string
		tags           []string
	}{
		{"indentation", "code:\n    x  :=  1\n#go", "code:\n    x  :=  1", []string{"go"}},
		{"line breaks", "line one\n#tag line two", "line one\nline two", []string{"tag"}},
		{"double spaces", "a  b #t", "a  b", []string{"t"}},
		{"tag-only line", "first\n#a #b\nlast", "first\nlast", []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, tags := tagpicker.StripHashtags(tc.in)
			if got != tc.want {
				t.Errorf("cleaned = %q, want %q", got, tc.want)
			}
			if !reflect.DeepEqual(tags, tc.tags) {
				t.Errorf("tags = %v, want %v", tags, tc.tags)
			}
		})
	}
}

// "#123" is an issue or PR reference, not a tag; "fixes #123" lost the
// reference and tagged the post "123".
func TestStripHashtags_NumbersAreNotTags(t *testing.T) {
	got, tags := tagpicker.StripHashtags("fixes #123")
	if got != "fixes #123" || tags != nil {
		t.Errorf("got %q %v, want the message unchanged and no tags", got, tags)
	}
}

// Code is quoted verbatim: "#include" in a C snippet is not a tag.
func TestStripHashtags_IgnoresCode(t *testing.T) {
	tests := []struct {
		name, in, want string
		tags           []string
	}{
		{"fenced block", "```\n#include <stdio.h>\n```", "```\n#include <stdio.h>\n```", nil},
		{"inline code", "use `#notatag` here #real", "use `#notatag` here", []string{"real"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, tags := tagpicker.StripHashtags(tc.in)
			if got != tc.want {
				t.Errorf("cleaned = %q, want %q", got, tc.want)
			}
			if !reflect.DeepEqual(tags, tc.tags) {
				t.Errorf("tags = %v, want %v", tags, tc.tags)
			}
		})
	}
}

func TestExtractHashtags_SkipsNumbersAndCode(t *testing.T) {
	tags := tagpicker.ExtractHashtags("see #42 and `#x` and #ok")
	if !reflect.DeepEqual(tags, []string{"ok"}) {
		t.Errorf("tags = %v, want [ok]", tags)
	}
}
