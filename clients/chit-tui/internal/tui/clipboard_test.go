package tui

import "testing"

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "single line", in: "hello", want: 1},
		{name: "two lines", in: "a\nb", want: 2},
		{name: "trailing newline counts the empty line", in: "a\n", want: 2},
		{name: "empty", in: "", want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countLines(tc.in); got != tc.want {
				t.Errorf("countLines(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// Copying nothing must not emit an escape sequence or claim a copy happened.
func TestCopyToClipboardIgnoresEmpty(t *testing.T) {
	if msg := copyToClipboard("")(); msg != nil {
		t.Errorf("copying an empty string returned %v, want nil", msg)
	}
}
