package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// handleKey routes a key press: open overlays first, then global
// bindings, then the focused component.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	assert.That(m.focus >= FocusViewport, "handleKey: focus before the first area")
	assert.That(m.appState == AppStateRunning, "handleKey: not signed in")
	assert.That(m.scope != nil, "handleKey: nil request scope")
	assert.That(m.focus <= FocusThreadInbox, "handleKey: focus past the last area")
	// Global keybindings (Ctrl+C always quits)
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if cmd, ok := m.answerQuestion(msg); ok {
		return m, cmd
	}
	if cmd, ok := m.routeOverlayKey(msg); ok {
		return m, cmd
	}
	if cmd, ok := m.globalKey(msg); ok {
		return m, cmd
	}
	if cmd, ok := m.viewportPostKey(msg); ok {
		return m, cmd
	}
	if key.Matches(msg, m.keys.TagPicker) {
		if cmd, ok := m.openTagPicker(); ok {
			return m, cmd
		}
	}

	if key.Matches(msg, m.keys.Tab) {
		return m, m.cycleFocus(1)
	}
	if key.Matches(msg, m.keys.ShiftTab) {
		return m, m.cycleFocus(-1)
	}

	// Delegate to focused component
	return m.delegateKey(msg)
}

// routeOverlayKey gives the key to a visible overlay or the mention popup.
func (m *Model) routeOverlayKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	assert.That(m.focus >= FocusViewport, "routeOverlayKey: focus before the first area")
	assert.That(m.focus <= FocusThreadInbox, "routeOverlayKey: focus past the last area")
	assert.That(m.appState == AppStateRunning, "routeOverlayKey: not signed in")
	// A visible overlay intercepts all keys. If it self-closes (Esc or a
	// selection), restore focus to a live component and run any
	// close-time behavior.
	for _, o := range m.overlays() {
		if !o.visible() {
			continue
		}
		cmd := m.updateOverlay(o.id, msg)
		assert.That(o.closeFocus >= focusKeep, "routeOverlayKey: unknown close focus")
		assert.That(o.closeFocus <= FocusThread, "routeOverlayKey: an overlay hands focus to another overlay")
		if !o.visible() && o.closeFocus != focusKeep {
			return tea.Batch(cmd, m.setFocus(o.closeFocus)), true
		}
		return cmd, true
	}

	// Intercept keys when mention popup is visible
	if m.mention.Visible() {
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyTab, tea.KeyEscape:
			var cmd tea.Cmd
			m.mention, cmd = m.mention.Update(msg)
			return cmd, true
		}
	}
	return nil, false
}

// globalKey handles the bindings that open a dialog or leave the current
// one, wherever focus is.
func (m *Model) globalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "globalKey: not signed in")
	assert.That(m.focus >= FocusViewport, "globalKey: focus before the first area")
	assert.That(m.focus <= FocusThreadInbox, "globalKey: focus past the last area")
	switch {
	case key.Matches(msg, m.keys.NewDM):
		return m.openPalette("@"), true
	case key.Matches(msg, m.keys.NewChannel) && m.focus != FocusChCreator && m.activeTeam != nil:
		return m.openChCreator(), true
	case key.Matches(msg, m.keys.CmdPalette):
		return m.openPalette(""), true
	case key.Matches(msg, m.keys.Search):
		return m.openPalette("?"), true
	// "?" is a printable key: only open help when not typing in a text box.
	case key.Matches(msg, m.keys.Help) && (m.focus == FocusViewport || m.focus == FocusThread):
		m.help.Open()
		return nil, true
	case key.Matches(msg, m.keys.Escape) && m.mainPane == paneThread:
		return m.closeThread(), true
	case key.Matches(msg, m.keys.Escape) && m.focus == FocusInput:
		m.cancelEdit()
		return m.setFocus(FocusViewport), true
	}
	return nil, false
}

// viewportPostKey acts on the post under the history cursor.
func (m *Model) viewportPostKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.focus != FocusViewport {
		return nil, false
	}
	assert.That(m.appState == AppStateRunning, "viewportPostKey: not signed in")
	assert.That(m.client != nil, "viewportPostKey: nil client")
	assert.That(m.clipboardOut != nil, "viewportPostKey: nil clipboard output")
	// Copy the mouse selection, or the post under the cursor when there
	// is none — so the key is useful without a mouse.
	if key.Matches(msg, m.keys.Copy) {
		if text := m.copyText(); text != "" {
			return copyToClipboard(m.clipboardOut, text), true
		}
	}
	// Editing loads the post back into the input; sending replaces it.
	// Only your own posts, matching what the server enforces.
	if key.Matches(msg, m.keys.Edit) {
		if p := m.ownSelectedPost(); p != nil {
			m.editingPostID = p.ID
			m.input.SetValue(p.Content)
			return m.setFocus(FocusInput), true
		}
	}
	// Deleting cannot be undone and "d" is one stray keystroke away, so
	// it asks first.
	if key.Matches(msg, m.keys.Delete) {
		if p := m.ownSelectedPost(); p != nil {
			return m.confirm.Ask(subjectDeletePost+p.ID, questionDeletePost, m.keys.Delete), true
		}
	}
	// Pinning is a channel-level act, so unlike edit and delete it works
	// on anyone's post.
	if key.Matches(msg, m.keys.Pin) {
		if p := m.viewport.SelectedPost(); p != nil {
			return SetPostPinned(m.reqCtx(), m.client, p.ID, !p.IsPinned), true
		}
	}
	return nil, false
}

// copyText is the mouse selection, or the content of the post under the
// cursor when there is none.
func (m Model) copyText() string {
	assert.That(m.focus == FocusViewport, "copyText: history pane not focused")
	assert.That(m.appState == AppStateRunning, "copyText: not signed in")
	if text := m.viewport.SelectedText(); text != "" {
		return text
	}
	if p := m.viewport.SelectedPost(); p != nil {
		return p.Content
	}
	return ""
}

// openTagPicker opens the tag picker on the post in focus, if there is one.
func (m *Model) openTagPicker() (tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "openTagPicker: not signed in")
	assert.That(m.postTags != nil, "openTagPicker: nil post tags")
	// In the history pane the subject is the post under the cursor;
	// in a thread there is no cursor, so it is the root post.
	var sel *model.Post
	switch m.focus {
	case FocusViewport:
		sel = m.viewport.SelectedPost()
	case FocusThread:
		sel = m.thread.RootPost()
	}
	if sel == nil {
		return nil, false
	}
	cmd := m.setFocus(FocusTagPicker)
	m.tagPicker.Open(sel.ID, m.tagsUsableIn(sel.ChannelID), m.postTags[sel.ID])
	assert.That(m.focus == FocusTagPicker, "openTagPicker: tag picker not focused")
	assert.That(m.tagPicker.Visible(), "openTagPicker: tag picker not open")
	return cmd, true
}
