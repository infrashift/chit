package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/actionbar"
)

// wheelScrollLines is how many lines one mouse-wheel notch scrolls.
const wheelScrollLines = 3

// handleMouse routes mouse events: wheel scrolls the main pane (or moves the
// palette cursor), left-clicks select palette rows or trigger action-bar
// buttons.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.palette.Visible() {
			m.palette.MoveCursor(-1)
		} else {
			m.scrollMain(-wheelScrollLines)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.palette.Visible() {
			m.palette.MoveCursor(1)
		} else {
			m.scrollMain(wheelScrollLines)
		}
		return m, nil
	}

	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Any click closes the help overlay.
	if m.help.Visible() {
		m.help.Close()
		return m, nil
	}

	if m.palette.Visible() {
		return m.handlePaletteClick(msg)
	}

	// Bar clicks are ignored while an overlay is open so a click can't stack
	// a second overlay on top of the first.
	if msg.Y == m.height-1 && !m.anyOverlayVisible() {
		m.syncActionBar()
		return m.dispatchAction(m.actionBar.HitTest(msg.X))
	}

	return m, nil
}

// handlePaletteClick selects the clicked palette row, or closes the palette
// when the click lands outside it.
func (m Model) handlePaletteClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	view := m.palette.View()
	px, py := m.paletteOrigin(view)
	w, h := lipgloss.Width(view), lipgloss.Height(view)

	inside := msg.X >= px && msg.X < px+w && msg.Y >= py && msg.Y < py+h
	if !inside {
		m.palette.Close()
		return m, m.setFocus(FocusInput)
	}

	if idx, ok := m.palette.RowAt(msg.Y - py); ok {
		cmd := m.palette.ChooseRow(idx)
		return m, cmd
	}
	return m, nil
}

// scrollMain scrolls whichever pane fills the top content area.
func (m *Model) scrollMain(lines int) {
	if m.mainPane == paneThread {
		m.thread.ScrollBy(lines)
		return
	}
	m.viewport.ScrollBy(lines)
}

// dispatchAction triggers the same code path as the action's keybinding.
func (m Model) dispatchAction(a actionbar.Action) (tea.Model, tea.Cmd) {
	switch a {
	case actionbar.ActionPalette:
		return m, m.openPalette("")
	case actionbar.ActionSearch:
		return m, m.openPalette("?")
	case actionbar.ActionPeople:
		return m, m.openPalette("@")
	case actionbar.ActionNewChannel:
		return m, m.openChCreator()
	case actionbar.ActionHelp:
		m.help.Open()
		return m, nil
	case actionbar.ActionCloseThread:
		if m.mainPane == paneThread {
			return m, m.closeThread()
		}
	}
	return m, nil
}

// anyOverlayVisible reports whether any floating overlay is currently open.
func (m Model) anyOverlayVisible() bool {
	for _, o := range m.overlays() {
		if o.visible() {
			return true
		}
	}
	return false
}
