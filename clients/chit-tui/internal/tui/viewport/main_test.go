package viewport_test

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain renders in full color. Without a terminal lipgloss drops every
// style, so a search highlight or selection is invisible and a test asking
// whether one was drawn cannot fail.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}
