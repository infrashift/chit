package help_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/help"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func newHelp() help.Model {
	m := help.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	return m
}

func TestHelp_HiddenByDefault(t *testing.T) {
	m := newHelp()
	if m.Visible() {
		t.Error("expected hidden by default")
	}
	if m.View() != "" {
		t.Error("expected empty view when hidden")
	}
}

func TestHelp_OpenShowsBindings(t *testing.T) {
	m := newHelp()
	m.Open()

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"Keyboard", "Mouse", "ctrl+k", "esc", "wheel", "press any key to close"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected %q in help view:\n%s", want, view)
		}
	}
}

func TestHelp_AnyKeyCloses(t *testing.T) {
	m := newHelp()
	m.Open()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.Visible() {
		t.Error("expected help to close on key press")
	}
}

// Replying is the capability people most often miss, because enter sends the
// message while the input is focused. The help must say how to get there.
func TestView_ExplainsHowToReply(t *testing.T) {
	m := help.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	m.Open()

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"reply in thread", "tab", "j/k"} {
		if !strings.Contains(view, want) {
			t.Errorf("help does not mention %q:\n%s", want, view)
		}
	}
}

// /theme and /logout never reach the server, so nothing else documents them.
func TestView_ListsClientCommands(t *testing.T) {
	m := help.New(styles.New(theme.TokyoNight()))
	m.SetSize(120, 40)
	m.Open()

	view := testutil.StripANSI(m.View())
	for _, want := range []string{"/theme", "/logout"} {
		if !strings.Contains(view, want) {
			t.Errorf("help omits %q:\n%s", want, view)
		}
	}
}
