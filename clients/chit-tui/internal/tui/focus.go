package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) setFocus(area FocusArea) tea.Cmd {
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
	return nil
}

// openChCreator opens the channel creator overlay for the active team.
func (m *Model) openChCreator() tea.Cmd {
	if m.activeTeam == nil {
		return nil
	}
	cmd := m.setFocus(FocusChCreator)
	m.chCreator.Open(m.activeTeam.ID)
	return cmd
}

// openPalette opens the unified palette with the query pre-filled to select
// a mode ("" channels, "@" people, "/" commands, "?" search).
func (m *Model) openPalette(prefix string) tea.Cmd {
	cmd := m.setFocus(FocusPalette)
	m.palette.SetActiveChannel(m.activeChannelDisplayName())
	m.palette.Open(prefix)
	return cmd
}

func (m *Model) cycleFocus(dir int) tea.Cmd {
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
	return m.setFocus(areas[next])
}

// delegateKey routes a key to the focused main component. Overlay focus
// areas never reach here: a visible overlay intercepts keys earlier in
// Update, and closing one restores focus to a main component.
func (m Model) delegateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
