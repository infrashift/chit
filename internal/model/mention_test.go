package model

import (
	"reflect"
	"testing"
)

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "basic mentions",
			content: "hello @alice and @bob",
			want:    []string{"alice", "bob"},
		},
		{
			name:    "dedup",
			content: "@alice @alice",
			want:    []string{"alice"},
		},
		{
			name:    "at all keyword",
			content: "hey @all please review",
			want:    []string{"all"},
		},
		{
			name:    "at channel keyword",
			content: "hey @channel please review",
			want:    []string{"channel"},
		},
		{
			name:    "no mentions",
			content: "hello world",
			want:    nil,
		},
		{
			name:    "email not matched",
			content: "email user@example.com please",
			want:    nil,
		},
		{
			name:    "case insensitive",
			content: "hi @Alice",
			want:    []string{"alice"},
		},
		{
			name:    "punctuation boundary before",
			content: "(@alice)",
			want:    []string{"alice"},
		},
		{
			name:    "start of string",
			content: "@alice hello",
			want:    []string{"alice"},
		},
		{
			name:    "mention with dots and dashes",
			content: "cc @john.doe-jr",
			want:    []string{"john.doe-jr"},
		},
		{
			name:    "multiple special keywords",
			content: "@all and @channel",
			want:    []string{"all", "channel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentions(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMentions(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
