package styles_test

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func TestInputs_FollowTheTheme(t *testing.T) {
	s := styles.New(theme.TokyoNight())
	if s.TermBackground != "#1a1b26" || s.TermForeground != "#c0caf5" {
		t.Errorf("terminal colors = %q, %q", s.TermForeground, s.TermBackground)
	}
	ti := textinput.New()
	s.ThemeTextInput(&ti)
	if ti.TextStyle.GetForeground() != s.InputText.GetForeground() ||
		ti.PlaceholderStyle.GetForeground() != s.InputPlaceholder.GetForeground() ||
		ti.Cursor.Style.GetForeground() != s.InputCursor.GetForeground() {
		t.Error("the text input kept the colors of bubbles")
	}
	ta := textarea.New()
	s.ThemeTextArea(&ta)
	for name, st := range map[string]textarea.Style{"focused": ta.FocusedStyle, "blurred": ta.BlurredStyle} {
		if st.Text.GetForeground() != s.InputText.GetForeground() || st.Placeholder.GetForeground() != s.InputPlaceholder.GetForeground() {
			t.Errorf("the %s text area kept the colors of bubbles", name)
		}
		if st.CursorLine.GetBackground() != (lipgloss.NoColor{}) {
			t.Errorf("the %s cursor line has a background of its own", name)
		}
	}
}

// A theme with no background leaves the terminal's own.
func TestInputs_NoBackground(t *testing.T) {
	th := theme.TokyoNight()
	th.Background, th.Foreground = nil, nil
	s := styles.New(th)
	if s.TermBackground != "" || s.TermForeground != "" {
		t.Errorf("terminal colors = %q, %q; want none", s.TermForeground, s.TermBackground)
	}
}
