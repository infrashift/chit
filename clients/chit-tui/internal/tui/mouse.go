package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/actionbar"
)

// wheelScrollLines is how many lines one mouse-wheel notch scrolls.
const wheelScrollLines = 3

// handleMouse routes mouse events: wheel scrolls the main pane, left-clicks
// on the bottom action bar dispatch the corresponding action.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollMain(-wheelScrollLines)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollMain(wheelScrollLines)
		return m, nil
	}

	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Bar clicks are ignored while an overlay is open so a click can't stack
	// a second overlay on top of the first.
	if msg.Y == m.height-1 && !m.anyOverlayVisible() {
		m.syncActionBar()
		return m.dispatchAction(m.actionBar.HitTest(msg.X))
	}

	return m, nil
}

// scrollMain scrolls whichever pane the wheel should act on.
func (m *Model) scrollMain(lines int) {
	if m.thread.Visible() && m.focus == FocusThread {
		m.thread.ScrollBy(lines)
		return
	}
	m.viewport.ScrollBy(lines)
}

// dispatchAction triggers the same code path as the action's keybinding.
func (m Model) dispatchAction(a actionbar.Action) (tea.Model, tea.Cmd) {
	switch a {
	case actionbar.ActionPalette:
		return m, m.openCmdPalette()
	case actionbar.ActionSearch:
		return m, m.openSearch()
	case actionbar.ActionPeople:
		return m, m.openDMPicker()
	case actionbar.ActionNewChannel:
		return m, m.openChCreator()
	case actionbar.ActionCloseThread:
		if m.thread.Visible() {
			m.thread.SetVisible(false)
			cmd := m.setFocus(FocusViewport)
			m.resizeComponents()
			return m, cmd
		}
	}
	return m, nil
}

// anyOverlayVisible reports whether any floating overlay is currently open.
func (m Model) anyOverlayVisible() bool {
	return m.cmdPalette.Visible() || m.search.Visible() || m.dmPicker.Visible() ||
		m.skinPicker.Visible() || m.chCreator.Visible() || m.tagPicker.Visible()
}
