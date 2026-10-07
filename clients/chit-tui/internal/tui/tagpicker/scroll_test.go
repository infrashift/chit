package tagpicker_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
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

func TestTagPicker_ScrollsToTheCursor(t *testing.T) {
	m := tagpicker.New(styles.New(theme.TokyoNight()))
	m.SetSize(100, 20)
	tags := make([]*model.Tag, scrollRows)
	for i := range tags {
		tags[i] = &model.Tag{ID: fmt.Sprint(i), Name: fmt.Sprintf("tag%02d", i)}
	}
	m.Open("p1", tags, nil)
	for _, k := range down(scrollRows - 1) {
		m, _ = m.Update(k)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "#tag29") {
		t.Errorf("the cursor's row is not on screen:\n%s", view)
	}
}
