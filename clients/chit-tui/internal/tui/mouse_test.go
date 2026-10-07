package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
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

func TestModel_MouseClickBarOpensPalette(t *testing.T) {
	m := setupModel(t)

	// Bar is the last row; the first button "[^K Jump]" spans columns [1,10).
	updated, _ := m.Update(tea.MouseMsg{X: 2, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "↵ select") {
		t.Errorf("expected palette open after bar click:\n%s", view)
	}
}

func TestModel_MouseClickBarOpensSearchMode(t *testing.T) {
	m := setupModel(t)

	// Second button "[^S Search]" spans columns [11,22).
	updated, _ := m.Update(tea.MouseMsg{X: 12, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Search in General") {
		t.Errorf("expected palette search mode after bar click:\n%s", view)
	}
}

func TestModel_MouseClickOutsidePaletteCloses(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)

	// Click far outside the palette (bottom bar area) — closes the palette.
	updated, _ = m.Update(tea.MouseMsg{X: 12, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "↵ select") {
		t.Errorf("expected palette to close on outside click:\n%s", view)
	}
}

func TestModel_MouseClickPaletteRowSelectsChannel(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)

	// The palette is centered at y=2; content rows start after the chrome
	// (2) plus input and context lines (2). Row 0 is "General".
	pv := testutil.StripANSI(m.View())
	if !strings.Contains(pv, "General") {
		t.Fatalf("expected General row in palette:\n%s", pv)
	}
	updated, cmd := m.Update(tea.MouseMsg{X: 60, Y: 6, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)
	wantMsg[palette.ChannelChosenMsg](t, cmd)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "↵ select") {
		t.Errorf("expected palette closed after row click:\n%s", view)
	}
}

func TestModel_MouseClickBarIgnoredWhenOverlayOpen(t *testing.T) {
	m := setupModel(t)

	// Open the channel creator (a non-palette overlay).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)

	// Click the search button on the bar — must not stack a second overlay.
	updated, _ = m.Update(tea.MouseMsg{X: 12, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "↵ select") {
		t.Errorf("expected bar click to be ignored while channel creator is open:\n%s", view)
	}
	if !strings.Contains(view, "Create Channel") {
		t.Errorf("expected channel creator to remain open:\n%s", view)
	}
}

func TestModel_MouseWheelMovesPaletteCursor(t *testing.T) {
	m := setupModel(t)

	// Two channels so the cursor has somewhere to go.
	updated, _ := m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", DisplayName: "General", TeamID: "t1"},
		{ID: "c2", DisplayName: "Random", TeamID: "t1"},
	}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	before := m.View()

	updated, _ = m.Update(tea.MouseMsg{X: 60, Y: 6, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = updated.(tui.Model)

	if after := m.View(); after == before {
		t.Error("expected palette cursor to move on wheel scroll")
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

// The wheel used to scroll the history no matter where the pointer was, so
// scrolling over the input box moved the wrong pane.
func TestMouse_WheelOverInputDoesNotScrollHistory(t *testing.T) {
	m := setupModel(t)

	before := testutil.StripANSI(m.View())

	// A row inside the input box, which sits below the history pane.
	updated, _ := m.Update(tea.MouseMsg{
		X: 10, Y: 37, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp,
	})
	m = updated.(tui.Model)

	if testutil.StripANSI(m.View()) != before {
		t.Error("wheel over the input scrolled the history")
	}
}

// Press anchors a selection and picks the post under the pointer, so a plain
// click behaves like choosing a post.
func TestMouse_PressInHistorySelectsPost(t *testing.T) {
	m := setupModel(t)
	// Newest first, as the API returns them: the top row is the oldest, and
	// the newest is the one selected by default.
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: posts("c1", "newest", "oldest")})

	m, _ = step(t, m, tea.MouseMsg{X: 5, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})

	if got := m.SelectedPostID(); got != "c1-oldest" {
		t.Errorf("selected %q, want the post under the pointer", got)
	}
}

// Motion after a press extends the selection; without it a drag does nothing.
func TestMouse_DragExtendsSelection(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.MouseMsg{
		X: 5, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.MouseMsg{
		X: 5, Y: 4, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft,
	})
	m = updated.(tui.Model)

	if !m.HasSelection() {
		t.Error("dragging did not produce a selection")
	}
}
