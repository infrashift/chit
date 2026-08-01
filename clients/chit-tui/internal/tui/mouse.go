package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/actionbar"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
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
			return m, nil
		}
		m.scrollUnderPointer(msg.Y, -wheelScrollLines)
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.palette.Visible() {
			m.palette.MoveCursor(1)
			return m, nil
		}
		m.scrollUnderPointer(msg.Y, wheelScrollLines)
		return m, nil
	}

	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Press, motion and release inside the history pane drive text selection.
	// They are handled before the release-only paths below, which cover the
	// action bar and overlays.
	if !m.anyOverlayVisible() && m.mainPane == paneChannel {
		if row, ok := m.historyContentRow(msg.Y); ok {
			return m.handleHistoryDrag(msg, row)
		}
	}

	if msg.Action != tea.MouseActionRelease {
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

// historyContentRow converts a screen row to a row inside the history pane's
// content area, reporting false when the pointer is elsewhere. The pane is
// drawn with a one-cell border, so the content starts one row in.
func (m Model) historyContentRow(y int) (int, bool) {
	const borderRows = 1
	top := borderRows
	// The input box and action bar occupy the bottom of the screen.
	bottom := m.height - inputHeight - 1 - borderRows
	if y < top || y >= bottom {
		return 0, false
	}
	return y - top, true
}

// handleHistoryDrag turns press/motion/release into a line selection. A press
// anchors it and also moves the post cursor, so a plain click behaves like
// picking a post; motion extends it; release leaves it in place to be copied.
func (m Model) handleHistoryDrag(msg tea.MouseMsg, contentRow int) (tea.Model, tea.Cmd) {
	line := m.viewport.LineAt(contentRow)

	switch msg.Action {
	case tea.MouseActionPress:
		m.viewport.SetSelectionAnchor(line)
		m.viewport.SelectPostAtLine(line)
		return m, m.setFocus(FocusViewport)
	case tea.MouseActionMotion:
		m.viewport.ExtendSelection(line)
		return m, nil
	case tea.MouseActionRelease:
		return m, nil
	}
	return m, nil
}

// scrollUnderPointer scrolls the pane the pointer is over. Previously the
// wheel always scrolled the main pane, so scrolling with the pointer over the
// input box moved the history instead.
func (m *Model) scrollUnderPointer(y, lines int) {
	if _, overHistory := m.historyContentRow(y); overHistory {
		m.scrollMain(lines)
	}
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
	case actionbar.ActionReply:
		// Same path the enter key takes in the history pane.
		if p := m.viewport.SelectedPost(); p != nil {
			return m, func() tea.Msg { return viewport.PostSelectedMsg{Post: p} }
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
