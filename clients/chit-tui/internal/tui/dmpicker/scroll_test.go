package dmpicker_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// Every picker drew its list from the first row, capped at a few rows, so
// moving the cursor past the cap moved it out of sight. The last row must be
// on screen once the cursor reaches it.

const scrollRows = 30

func down(n int) []tea.KeyMsg {
	keys := make([]tea.KeyMsg, n)
	for i := range keys {
		keys[i] = tea.KeyMsg{Type: tea.KeyDown}
	}
	return keys
}

func TestDMPicker_ScrollsToTheCursor(t *testing.T) {
	m := dmpicker.New(styles.New(theme.TokyoNight()))
	m.SetSize(100, 20)
	m.OpenForMembers()
	users := make([]*model.User, scrollRows)
	for i := range users {
		users[i] = &model.User{ID: fmt.Sprint(i), Username: fmt.Sprintf("user%02d", i)}
	}
	m.SetResults(users)
	for _, k := range down(scrollRows - 1) {
		m, _ = m.Update(k)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "@user29") {
		t.Errorf("the cursor's row is not on screen:\n%s", view)
	}
}
