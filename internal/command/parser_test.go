package command

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantOK  bool
		slug    string
		args    string
	}{
		{"valid slug only", "/help", true, "help", ""},
		{"valid with args", "/invite @alice", true, "invite", "@alice"},
		{"valid with multiple args", "/topic New topic here", true, "topic", "New topic here"},
		{"slug with numbers", "/cmd123", true, "cmd123", ""},
		{"slug with dashes", "/my-cmd", true, "my-cmd", ""},
		{"empty string", "", false, "", ""},
		{"no slash", "help", false, "", ""},
		{"slash alone", "/", false, "", ""},
		{"invalid chars in slug", "/Foo_bar", false, "", ""},
		{"space before slash", " /help", false, "", ""},
		{"regular message", "hello world", false, "", ""},
		{"message with slash later", "see /help for info", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := Parse(tt.content)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q): got ok=%v, want %v", tt.content, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if result.Slug != tt.slug {
				t.Errorf("Parse(%q): slug=%q, want %q", tt.content, result.Slug, tt.slug)
			}
			if result.Args != tt.args {
				t.Errorf("Parse(%q): args=%q, want %q", tt.content, result.Args, tt.args)
			}
		})
	}
}
