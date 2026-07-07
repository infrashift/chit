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
