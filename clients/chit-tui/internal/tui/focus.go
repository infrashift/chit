package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

func (m *Model) setFocus(area FocusArea) tea.Cmd {
	assert.That(area >= FocusViewport, "setFocus: focus area before the first")
	assert.That(area <= FocusThreadInbox, "setFocus: focus area past the last")
	// Blurring the pane about to be focused again would redraw it twice for
	// nothing; the history pane redraws its whole content to do it.
	if area != FocusViewport {
		m.viewport.Blur()
	}
	if area != FocusInput {
		m.input.Blur()
	}
	if area != FocusThread {
		m.thread.Blur()
	}
	for _, o := range m.overlays() {
		o.blur()
	}

	m.focus = area
	switch area {
	case FocusViewport:
		m.viewport.Focus()
	case FocusInput:
		return m.input.Focus()
	case FocusThread:
		m.thread.Focus()
	case FocusPalette:
		m.palette.Focus()
	case FocusDMPicker:
		m.dmPicker.Focus()
	case FocusSkinPicker:
		m.skinPicker.Focus()
	case FocusChCreator:
		m.chCreator.Focus()
	case FocusTagPicker:
		m.tagPicker.Focus()
	case FocusThreadInbox:
		m.threadInbox.Focus()
	}
	assert.That(m.focus == area, "setFocus: focus not moved")
	return nil
}

// openChCreator opens the channel creator overlay for the active team.
func (m *Model) openChCreator() tea.Cmd {
	assert.That(m.appState == AppStateRunning, "openChCreator: not signed in")
	if m.activeTeam == nil {
		return nil
	}
	cmd := m.setFocus(FocusChCreator)
	m.chCreator.Open(m.activeTeam.ID)
	assert.That(m.focus == FocusChCreator, "openChCreator: creator not focused")
	assert.That(m.chCreator.Visible(), "openChCreator: creator not open")
	return cmd
}

// openPalette opens the unified palette with the query pre-filled to select
// a mode ("" channels, "@" people, "/" commands, "?" search).
func (m *Model) openPalette(prefix string) tea.Cmd {
	assert.That(m.appState == AppStateRunning, "openPalette: not signed in")
	assert.That(len(prefix) <= 1, "openPalette: prefix longer than a mode character")
	cmd := m.setFocus(FocusPalette)
	m.palette.SetActiveChannel(m.activeChannelDisplayName())
	m.palette.Open(prefix)
	assert.That(m.focus == FocusPalette, "openPalette: palette not focused")
	assert.That(m.palette.Visible(), "openPalette: palette not open")
	return cmd
}

func (m *Model) cycleFocus(dir int) tea.Cmd {
	assert.That(m.appState == AppStateRunning, "cycleFocus: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "cycleFocus: unknown main pane")
	assert.That(dir == 1 || dir == -1, "cycleFocus: step is not one area")
	areas := []FocusArea{FocusViewport, FocusInput}
	if m.mainPane == paneThread {
		areas = []FocusArea{FocusThread, FocusInput}
	}

	current := 0
	for i, a := range areas {
		if a == m.focus {
			current = i
			break
		}
	}

	next := (current + dir + len(areas)) % len(areas)
	assert.That(len(areas) == 2, "cycleFocus: cycling other than two panes")
	assert.That(next >= 0, "cycleFocus: negative area index")
	assert.That(next < len(areas), "cycleFocus: area index past the panes")
	return m.setFocus(areas[next])
}

// delegateKey routes a key to the focused main component. Overlay focus
// areas never reach here: a visible overlay intercepts keys earlier in
// Update, and closing one restores focus to a main component.
func (m Model) delegateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	assert.That(!m.anyOverlayVisible(), "delegateKey: an overlay is open")
	assert.That(m.focus >= FocusViewport, "delegateKey: focus before the first area")
	assert.That(m.focus <= FocusThreadInbox, "delegateKey: focus past the last area")
	var cmd tea.Cmd
	switch m.focus {
	case FocusViewport:
		m.viewport, cmd = m.viewport.Update(msg)
		// Scrolling or moving the cursor may have reached the oldest loaded
		// post, which is the cue to fetch the page before it.
		if older := m.maybeLoadOlder(); older != nil {
			return m, tea.Batch(cmd, older)
		}
	case FocusInput:
		m.input, cmd = m.input.Update(msg)
	case FocusThread:
		m.thread, cmd = m.thread.Update(msg)
	}
	return m, cmd
}
