package testutil_test

import (
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
)

func TestStripANSI(t *testing.T) {
	tests := map[string]string{
		"color":     "\x1b[38;2;1;2;3mhello\x1b[0m",
		"cursor":    "\x1b[2Khello",
		"hyperlink": "\x1b]8;;https://example.com\x1b\\hello\x1b]8;;\x1b\\",
	}
	for name, in := range tests {
		if got := testutil.StripANSI(in); got != "hello" {
			t.Errorf("%s: StripANSI(%q) = %q, want hello", name, in, got)
		}
	}
}
