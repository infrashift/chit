package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
)

// setupModelWithManyPosts loads enough posts that the viewport can scroll.
func setupModelWithManyPosts(t *testing.T) tui.Model {
	t.Helper()
	m := setupModel(t)
	posts := make([]*model.Post, 60)
	for i := range posts {
		posts[i] = &model.Post{
			ID:       fmt.Sprintf("p%d", i),
			UserID:   "u1",
			Content:  fmt.Sprintf("message number %d", i),
			CreateAt: 1700000000000 + int64(i)*1000,
		}
	}
	updated, _ := m.Update(tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: posts}})
	return updated.(tui.Model)
}

func TestModel_MouseWheelScrollsViewport(t *testing.T) {
	m := setupModelWithManyPosts(t)
	before := m.View()

	updated, _ := m.Update(tea.MouseMsg{X: 60, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	m = updated.(tui.Model)

	if after := m.View(); after == before {
		t.Error("expected view to change after wheel-up scroll")
	}
}

func TestModel_MouseWheelDownAfterUpRestoresBottom(t *testing.T) {
	m := setupModelWithManyPosts(t)
	bottom := m.View()

	updated, _ := m.Update(tea.MouseMsg{X: 60, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.MouseMsg{X: 60, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = updated.(tui.Model)

	if got := m.View(); got != bottom {
		t.Error("expected wheel down to scroll back to the bottom view")
	}
}

func TestModel_MouseClickBarOpensCmdPalette(t *testing.T) {
	m := setupModel(t)

	// Bar is the last row; the first button "[^K Jump]" spans columns [1,10).
	updated, _ := m.Update(tea.MouseMsg{X: 2, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Search commands") {
		t.Errorf("expected command palette open after bar click:\n%s", view)
	}
}

func TestModel_MouseClickBarOpensSearch(t *testing.T) {
	m := setupModel(t)

	// Second button "[^S Search]" spans columns [11,22).
	updated, _ := m.Update(tea.MouseMsg{X: 12, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Search posts") {
		t.Errorf("expected search overlay open after bar click:\n%s", view)
	}
}

func TestModel_MouseClickBarIgnoredWhenOverlayOpen(t *testing.T) {
	m := setupModel(t)

	// Open the command palette via its key first.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)

	// Click the search button on the bar — must not stack a second overlay.
	updated, _ = m.Update(tea.MouseMsg{X: 12, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "Search posts") {
		t.Errorf("expected bar click to be ignored while palette is open:\n%s", view)
	}
	if !strings.Contains(view, "Search commands") {
		t.Errorf("expected command palette to remain open:\n%s", view)
	}
}

func TestModel_MouseClickOutsideBarIsNoOp(t *testing.T) {
	m := setupModel(t)
	before := m.View()

	updated, _ := m.Update(tea.MouseMsg{X: 60, Y: 20, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	if after := m.View(); after != before {
		t.Error("expected click outside the bar to be a no-op")
	}
}

func TestModel_ActionBarShowsConnectionState(t *testing.T) {
	m := setupModel(t)

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "○") {
		t.Errorf("expected disconnected indicator before WS connect:\n%s", view)
	}

	updated, _ := m.Update(tui.WSConnectedMsg{})
	m = updated.(tui.Model)

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "●") {
		t.Errorf("expected connected indicator after WSConnectedMsg:\n%s", view)
	}
}
