package tui

import (
	"errors"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// handleKey routes a key press: open overlays first, then global
// bindings, then the focused component.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keybindings (Ctrl+C always quits)
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	// A pending delete is confirmed by pressing the key again on the
	// same post, while the question is still on screen; once it has
	// timed out or been replaced, nothing is being asked. Anything else
	// cancels it and then does what it does.
	if id := m.confirmDeleteID; id != "" {
		m.confirmDeleteID = ""
		asking := errors.Is(m.err, errConfirmDelete)
		if asking {
			m.err = nil
		}
		if asking && key.Matches(msg, m.keys.Delete) && m.focus == FocusViewport {
			if p := m.ownSelectedPost(); p != nil && p.ID == id {
				return m, DeletePost(m.reqCtx(), m.client, p.ID)
			}
		}
	}

	// A visible overlay intercepts all keys. If it self-closes (Esc or a
	// selection), restore focus to a live component and run any
	// close-time behavior.
	for _, o := range m.overlays() {
		if !o.visible() {
			continue
		}
		cmd := o.update(msg)
		if !o.visible() {
			if o.closeFocus != focusKeep {
				return m, tea.Batch(cmd, m.setFocus(o.closeFocus))
			}
			return m, cmd
		}
		return m, cmd
	}

	// Intercept keys when mention popup is visible
	if m.mention.Visible() {
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyTab, tea.KeyEscape:
			var cmd tea.Cmd
			m.mention, cmd = m.mention.Update(msg)
			return m, cmd
		}
	}

	if key.Matches(msg, m.keys.NewDM) {
		return m, m.openPalette("@")
	}

	if key.Matches(msg, m.keys.NewChannel) && m.focus != FocusChCreator && m.activeTeam != nil {
		return m, m.openChCreator()
	}

	if key.Matches(msg, m.keys.CmdPalette) {
		return m, m.openPalette("")
	}

	if key.Matches(msg, m.keys.Search) {
		return m, m.openPalette("?")
	}

	// "?" is a printable key: only open help when not typing in a text box.
	if key.Matches(msg, m.keys.Help) && (m.focus == FocusViewport || m.focus == FocusThread) {
		m.help.Open()
		return m, nil
	}

	if key.Matches(msg, m.keys.Escape) {
		if m.mainPane == paneThread {
			return m, m.closeThread()
		}
		if m.focus == FocusInput {
			m.cancelEdit()
			return m, m.setFocus(FocusViewport)
		}
	}

	// Copy the mouse selection, or the post under the cursor when there
	// is none — so the key is useful without a mouse.
	if key.Matches(msg, m.keys.Copy) && m.focus == FocusViewport {
		text := m.viewport.SelectedText()
		if text == "" {
			if p := m.viewport.SelectedPost(); p != nil {
				text = p.Content
			}
		}
		if text != "" {
			return m, copyToClipboard(text)
		}
	}

	// Editing loads the post back into the input; sending replaces it.
	// Only your own posts, matching what the server enforces.
	if key.Matches(msg, m.keys.Edit) && m.focus == FocusViewport {
		if p := m.ownSelectedPost(); p != nil {
			m.editingPostID = p.ID
			m.input.SetValue(p.Content)
			return m, m.setFocus(FocusInput)
		}
	}

	// Deleting cannot be undone and "d" is one stray keystroke away, so
	// it asks first.
	if key.Matches(msg, m.keys.Delete) && m.focus == FocusViewport {
		if p := m.ownSelectedPost(); p != nil {
			m.confirmDeleteID = p.ID
			return m, m.setError(errConfirmDelete)
		}
	}

	// Pinning is a channel-level act, so unlike edit and delete it works
	// on anyone's post.
	if key.Matches(msg, m.keys.Pin) && m.focus == FocusViewport {
		if p := m.viewport.SelectedPost(); p != nil {
			return m, SetPostPinned(m.reqCtx(), m.client, p.ID, !p.IsPinned)
		}
	}

	if key.Matches(msg, m.keys.TagPicker) {
		// In the history pane the subject is the post under the cursor;
		// in a thread there is no cursor, so it is the root post.
		var sel *model.Post
		switch m.focus {
		case FocusViewport:
			sel = m.viewport.SelectedPost()
		case FocusThread:
			sel = m.thread.RootPost()
		}
		if sel != nil {
			cmd := m.setFocus(FocusTagPicker)
			m.tagPicker.Open(sel.ID, m.allTags, m.postTags[sel.ID])
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
