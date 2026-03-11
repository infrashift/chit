package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPlaceOverlay_BasicASCII(t *testing.T) {
	bg := "AAAAAAAAAA\nBBBBBBBBBB\nCCCCCCCCCC"
	fg := "xx\nyy"
	got := placeOverlay(2, 1, fg, bg)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "AAAAAAAAAA" {
		t.Errorf("line 0 = %q, want %q", lines[0], "AAAAAAAAAA")
	}
	if lines[1] != "BBxxBBBBBB" {
		t.Errorf("line 1 = %q, want %q", lines[1], "BBxxBBBBBB")
	}
	if lines[2] != "CCyyCCCCCC" {
		t.Errorf("line 2 = %q, want %q", lines[2], "CCyyCCCCCC")
	}
}

func TestPlaceOverlay_ANSIAware(t *testing.T) {
	// Create a styled background line using lipgloss
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#C0CAF5"))
	bgLine := style.Render("AAAAAAAAAA")
	bg := bgLine + "\n" + bgLine + "\n" + bgLine

	fg := "XX"
	got := placeOverlay(2, 1, fg, bg)

	// The result should NOT contain raw ANSI fragments like "38;2;192" as visible text.
	// If ANSI sequences were broken, digits would leak into visible output.
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		// Strip all ANSI escape sequences and check no raw escape fragments
		stripped := stripANSI(line)
		if strings.Contains(stripped, "38;") || strings.Contains(stripped, "2;192") {
			t.Errorf("line %d contains raw ANSI fragments: %q", i, stripped)
		}
	}

	// The overlay line should contain "XX"
	if !strings.Contains(lines[1], "XX") {
		t.Errorf("overlay line should contain 'XX', got %q", lines[1])
	}
}

func TestPlaceOverlay_ForegroundWiderThanBackground(t *testing.T) {
	bg := "AAA\nBBB"
	fg := "XXXXXXXX"
	// Should not panic
	got := placeOverlay(0, 0, fg, bg)
	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[0], "XXXXXXXX") {
		t.Errorf("line 0 should contain full overlay, got %q", lines[0])
	}
	if lines[1] != "BBB" {
		t.Errorf("line 1 = %q, want %q", lines[1], "BBB")
	}
}

func TestPlaceOverlay_OverlayAtEdge(t *testing.T) {
	bg := "AAAAAAAAAA\nBBBBBBBBBB"
	fg := "XX\nYY"
	got := placeOverlay(0, 0, fg, bg)
	lines := strings.Split(got, "\n")
	if lines[0] != "XXAAAAAAAA" {
		t.Errorf("line 0 = %q, want %q", lines[0], "XXAAAAAAAA")
	}
	if lines[1] != "YYBBBBBBBB" {
		t.Errorf("line 1 = %q, want %q", lines[1], "YYBBBBBBBB")
	}
}

func TestPlaceOverlay_BackgroundShorterThanX(t *testing.T) {
	bg := "AAA\nBBB"
	fg := "XX"
	got := placeOverlay(5, 0, fg, bg)
	lines := strings.Split(got, "\n")
	// Background is only 3 wide, overlay at x=5 should pad with spaces
	if lines[0] != "AAA  XX" {
		t.Errorf("line 0 = %q, want %q", lines[0], "AAA  XX")
	}
	if lines[1] != "BBB" {
		t.Errorf("line 1 = %q, want %q", lines[1], "BBB")
	}
}

func TestPlaceOverlay_BeyondHeight(t *testing.T) {
	bg := "AAA\nBBB"
	fg := "XX\nYY\nZZ"
	got := placeOverlay(0, 1, fg, bg)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "AAA" {
		t.Errorf("line 0 = %q, want %q", lines[0], "AAA")
	}
	if lines[1] != "XXB" {
		t.Errorf("line 1 = %q, want %q", lines[1], "XXB")
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find the terminating letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the terminating letter
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
