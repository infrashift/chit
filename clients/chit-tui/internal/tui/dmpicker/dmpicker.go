// Package dmpicker implements the member-selection overlay used when
// creating a private channel. (Starting DMs is handled by the palette's
// "@" mode.)
package dmpicker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
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
	return Model{
		input:  ti,
		styles: s,
	}
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
}

// Close hides the member picker overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	m.results = nil
	m.selected = nil
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
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
	m.results = users
	m.cursor = 0
}

// isSelected checks if a user is in the selected list.
func (m Model) isSelected(userID string) bool {
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
	for i, u := range m.selected {
		if u.ID == user.ID {
			m.selected = append(m.selected[:i], m.selected[i+1:]...)
			return false
		}
	}
	if len(m.selected) < maxGroupSelection {
		m.selected = append(m.selected, user)
		return true
	}
	return false
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.Type {
		case tea.KeyEscape:
			m.Close()
			return m, func() tea.Msg { return CancelledMsg{} }
		case tea.KeyTab:
			if len(m.results) > 0 && m.cursor < len(m.results) {
				if m.toggleSelected(m.results[m.cursor]) {
					// The picked name is a chip now, so the box belongs to
					// the next one. Leaving it filled makes the following
					// term concatenate onto it and match nobody.
					//
					// The results stay: several people often come back from
					// one search, and Tab down the list must keep working.
					m.input.Reset()
					m.lastSearched = ""
				}
			}
			return m, nil
		case tea.KeyEnter:
			// A fresh term searches, even with people already selected.
			// Confirming instead would make a second search impossible and
			// cap every pick at whatever one query happened to return —
			// which no group of three can rely on.
			if term := m.input.Value(); term != "" && term != m.lastSearched {
				m.lastSearched = term
				return m, func() tea.Msg { return SearchTriggeredMsg{Term: term} }
			}
			if len(m.selected) > 0 {
				users := make([]*model.User, len(m.selected))
				copy(users, m.selected)
				m.Close()
				return m, func() tea.Msg { return MembersPickedMsg{Users: users} }
			}
			if len(m.results) > 0 && m.cursor < len(m.results) {
				user := m.results[m.cursor]
				users := []*model.User{user}
				m.Close()
				return m, func() tea.Msg { return MembersPickedMsg{Users: users} }
			}
			return m, nil
		case tea.KeyBackspace:
			if m.input.Value() == "" && len(m.selected) > 0 {
				m.selected = m.selected[:len(m.selected)-1]
				return m, nil
			}
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the member picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

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

	maxItems := min(len(m.results), max((m.height/2)-4, 5))
	for i := range maxItems {
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

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
