// Package dmpicker is the member-selection overlay, used to add members to a
// new private channel and to choose the people for /group. (Starting a DM
// is the palette's "@" mode.)
package dmpicker

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

const maxGroupSelection = 7 // cap on members selectable in one picker session

// MembersPickedMsg is sent when members are selected for a private channel.
type MembersPickedMsg struct {
	Users []*model.User
}

// CancelledMsg is sent when the picker is dismissed without selecting
// members (Esc).
type CancelledMsg struct{}

// SearchTriggeredMsg is sent when a search should be performed.
type SearchTriggeredMsg struct {
	Term string
}

// Model is the member picker overlay component.
type Model struct {
	input    textinput.Model
	results  []*model.User
	selected []*model.User
	// lastSearched is the term the current results came from, so Enter can
	// tell "search this" apart from "I am done picking".
	lastSearched string
	cursor       int
	visible      bool
	focused      bool
	styles       styles.Styles
	width        int
	height       int
}

// New creates a new member picker model.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Search users..."
	ti.CharLimit = 256
	m := Model{
		input:  ti,
		styles: s,
	}
	assert.That(m.focused == m.input.Focused(), "dmpicker.New: focus flag and search box disagree")
	return m
}

// OpenForMembers shows the picker for private-channel member selection.
func (m *Model) OpenForMembers() {
	m.visible = true
	m.focused = true
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.results = nil
	m.selected = nil
	m.lastSearched = ""
	assert.That(m.input.Focused(), "Model.OpenForMembers: search box not focused")
	assert.That(m.input.Value() == "", "Model.OpenForMembers: search box not cleared")
}

// Close hides the member picker overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	m.results = nil
	m.selected = nil
	assert.That(!m.input.Focused(), "Model.Close: search box kept focus")
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
	assert.That(m.input.Focused(), "Model.Focus: search box not focused")
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
	assert.That(!m.input.Focused(), "Model.Blur: search box kept focus")
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w - 8
}

// SetResults sets the user search results.
func (m *Model) SetResults(users []*model.User) {
	assert.That(m.visible, "Model.SetResults: results for a closed picker")
	assert.That(len(m.selected) <= maxGroupSelection, "Model.SetResults: too many people selected")
	m.results = users
	m.cursor = 0
}

// isSelected checks if a user is in the selected list.
func (m Model) isSelected(userID string) bool {
	assert.That(len(m.selected) <= maxGroupSelection, "Model.isSelected: too many people selected")
	for _, u := range m.selected {
		if u.ID == userID {
			return true
		}
	}
	return false
}

// toggleSelected adds or removes a user from the selected list, reporting
// whether the user ended up selected.
func (m *Model) toggleSelected(user *model.User) bool {
	assert.That(len(m.selected) <= maxGroupSelection, "Model.toggleSelected: too many people selected")
	for i, u := range m.selected {
		for _, v := range m.selected[i+1:] {
			assert.That(u.ID != v.ID, "Model.toggleSelected: a person selected twice")
		}
	}
	before := len(m.selected)
	for i, u := range m.selected {
		if u.ID == user.ID {
			m.selected = append(m.selected[:i], m.selected[i+1:]...)
			assert.That(len(m.selected) == before-1, "Model.toggleSelected: deselect did not remove one person")
			return false
		}
	}
	if len(m.selected) < maxGroupSelection {
		m.selected = append(m.selected, user)
		assert.That(len(m.selected) <= maxGroupSelection, "Model.toggleSelected: selection over the cap")
		assert.That(m.selected[len(m.selected)-1] == user, "Model.toggleSelected: picked person not last")
		return true
	}
	return false
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.results)-1, 0), "Model.Update: cursor outside the results")
	assert.That(m.input.Focused(), "Model.Update: focused picker with a blurred search box")

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if next, cmd, handled := m.handleKey(keyMsg); handled {
			return next, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	assert.That(utf8.RuneCountInString(m.input.Value()) <= m.input.CharLimit, "Model.Update: search term over the input limit")
	return m, cmd
}

// handleKey handles the keys the picker itself uses, and reports whether
// msg was one of them; the rest go to the search box.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	assert.That(m.visible && m.focused, "Model.handleKey: keys reached a closed or unfocused picker")
	switch msg.Type {
	case tea.KeyEscape:
		m.Close()
		assert.That(!m.visible && len(m.selected) == 0, "Model.handleKey: Esc left the picker open")
		return m, func() tea.Msg { return CancelledMsg{} }, true
	case tea.KeyTab:
		m.toggleCursor()
		return m, nil, true
	case tea.KeyEnter:
		next, cmd := m.confirm()
		return next, cmd, true
	case tea.KeyBackspace:
		if m.input.Value() == "" && len(m.selected) > 0 {
			m.selected = m.selected[:len(m.selected)-1]
			assert.That(len(m.selected) < maxGroupSelection, "Model.handleKey: Backspace left the selection full")
			return m, nil, true
		}
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		assert.That(m.cursor >= 0, "Model.handleKey: cursor above the results")
		return m, nil, true
	case tea.KeyDown:
		if m.cursor < len(m.results)-1 {
			m.cursor++
		}
		assert.That(m.cursor <= max(len(m.results)-1, 0), "Model.handleKey: cursor below the results")
		return m, nil, true
	}
	return m, nil, false
}

// toggleCursor selects or deselects the person under the cursor.
func (m *Model) toggleCursor() {
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.results)-1, 0), "Model.toggleCursor: cursor outside the results")
	if len(m.results) == 0 || m.cursor >= len(m.results) {
		return
	}
	assert.That(m.cursor >= 0, "Model.toggleCursor: negative cursor")
	if m.toggleSelected(m.results[m.cursor]) {
		// The picked name is a chip now, so the box belongs to
		// the next one. Leaving it filled makes the following
		// term concatenate onto it and match nobody.
		//
		// The results stay: several people often come back from
		// one search, and Tab down the list must keep working.
		m.input.Reset()
		m.lastSearched = ""
		assert.That(m.input.Value() == "", "Model.toggleCursor: search box not cleared after a pick")
	}
	assert.That(len(m.selected) <= maxGroupSelection, "Model.toggleCursor: selection over the cap")
}

// confirm searches for a new term, or else picks the selected people, or
// else the one under the cursor.
func (m Model) confirm() (Model, tea.Cmd) {
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.results)-1, 0), "Model.confirm: cursor outside the results")
	// A fresh term searches, even with people already selected.
	// Confirming instead would make a second search impossible and
	// cap every pick at whatever one query happened to return —
	// which no group of three can rely on.
	if term := m.input.Value(); term != "" && term != m.lastSearched {
		m.lastSearched = term
		return m, func() tea.Msg { return SearchTriggeredMsg{Term: term} }
	}
	assert.That(len(m.selected) <= maxGroupSelection, "Model.confirm: selection over the cap")
	if len(m.selected) > 0 {
		users := make([]*model.User, len(m.selected))
		n := copy(users, m.selected)
		assert.That(n == len(m.selected), "Model.confirm: selection not copied whole")
		m.Close()
		assert.That(!m.visible && len(users) == n, "Model.confirm: picker open or picks lost after closing")
		return m, func() tea.Msg { return MembersPickedMsg{Users: users} }
	}
	if len(m.results) > 0 && m.cursor < len(m.results) {
		assert.That(m.cursor >= 0, "Model.confirm: negative cursor")
		user := m.results[m.cursor]
		users := []*model.User{user}
		m.Close()
		return m, func() tea.Msg { return MembersPickedMsg{Users: users} }
	}
	return m, nil
}

// View renders the member picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.results)-1, 0), "Model.View: cursor outside the results")
	assert.That(len(m.selected) <= maxGroupSelection, "Model.View: selection over the cap")
	assert.That(m.focused == m.input.Focused(), "Model.View: focus flag and search box disagree")
	var items []string

	// Selected chips
	if len(m.selected) > 0 {
		var chips []string
		for _, u := range m.selected {
			chips = append(chips, m.styles.MentionText.Render("@"+u.Username))
		}
		items = append(items, strings.Join(chips, " "))
	}

	items = append(items, m.input.View())
	items = append(items, "")

	// The hint has to track what Enter will actually do, which now depends on
	// whether the box holds a term that has not been searched yet.
	switch term := m.input.Value(); {
	case term != "" && term != m.lastSearched:
		items = append(items, m.styles.Timestamp.Render("  Enter to search"))
	case len(m.selected) > 0:
		items = append(items, m.styles.Timestamp.Render(
			"  Tab to pick · Enter to confirm · type another name to keep searching"))
	case len(m.results) > 0:
		items = append(items, m.styles.Timestamp.Render("  Tab to pick · Enter to choose"))
	}

	start, end := listwin.Window(m.cursor, len(m.results), listwin.Rows(m.height))
	assert.That(start >= 0 && start <= end && end <= len(m.results), "Model.View: window outside the results")
	header := len(items)
	for i := start; i < end; i++ {
		u := m.results[i]
		label := fmt.Sprintf("@%s", u.Username)
		if u.DisplayName != "" {
			label += fmt.Sprintf(" (%s)", u.DisplayName)
		}
		if m.isSelected(u.ID) {
			label = "[x] " + label
		} else {
			label = "[ ] " + label
		}
		if i == m.cursor {
			label = m.styles.ListItemActive.Render("> " + label)
		} else {
			label = m.styles.ListItem.Render("  " + label)
		}
		items = append(items, label)
	}
	assert.That(len(items) == header+end-start, "Model.View: row count does not match the window")

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
