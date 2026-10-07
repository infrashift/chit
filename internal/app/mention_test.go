package app

import (
	"reflect"
	"sort"
	"testing"

	"github.com/infrashift/chit/internal/model"
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
			got := parseMentions(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMentions(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestProcessMentions(t *testing.T) {
	cases := []struct {
		name    string
		members []string
		content string
		want    []string
	}{
		{"resolves usernames to user IDs", []string{"author", "alice", "bob"}, "hey @alice and @bob", []string{"alice", "bob"}},
		{"skips self-mention", []string{"author", "alice"}, "I am @author and @alice", []string{"alice"}},
		{"skips non-members", []string{"author", "alice"}, "hey @alice and @charlie", []string{"alice"}},
		{"@all is every member but the author", []string{"author", "alice", "bob"}, "hey @all", []string{"alice", "bob"}},
		{"unknown username silently skipped", []string{"author", "alice"}, "hey @alice and @nonexistent", []string{"alice"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.join(f.channel, tc.members...)
			f.user("charlie") // exists, but is not in the channel

			ids, err := f.app.processMentions(t.Context(), &model.Post{
				ChannelID: f.channel.ID, UserID: f.user("author").ID, Content: tc.content,
			})
			if err != nil {
				t.Fatalf("processMentions: %v", err)
			}

			want := make([]string, 0, len(tc.want))
			for _, n := range tc.want {
				want = append(want, f.user(n).ID)
			}
			sort.Strings(ids)
			sort.Strings(want)
			if !reflect.DeepEqual(ids, want) {
				t.Errorf("got %v, want %v (%v)", ids, want, tc.want)
			}
		})
	}
}
