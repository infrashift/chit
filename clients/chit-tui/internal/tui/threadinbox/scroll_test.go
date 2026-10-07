package threadinbox_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
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

// The inbox had no cap at all, so a long list ran off the bottom.
func TestThreadInbox_FitsTheScreenAndScrolls(t *testing.T) {
	m := threadinbox.New(styles.New(theme.TokyoNight()))
	m.SetSize(100, 20)
	m.Open()
	threads := make([]*model.ThreadResponse, scrollRows)
	for i := range threads {
		threads[i] = &model.ThreadResponse{
			Thread: &model.Thread{PostID: fmt.Sprint(i), ChannelID: "c1"},
			Posts:  []*model.Post{{ID: fmt.Sprint(i), Content: fmt.Sprintf("topic %02d", i)}},
		}
	}
	m.SetThreads(threads, nil)

	if h := strings.Count(m.View(), "\n") + 1; h > 20 {
		t.Errorf("inbox is %d rows tall on a 20-row terminal", h)
	}
	for _, k := range down(scrollRows - 1) {
		m, _ = m.Update(k)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "topic 29") {
		t.Errorf("the cursor's row is not on screen:\n%s", view)
	}
}
