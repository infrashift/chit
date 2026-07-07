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

const maxGroupSelection = 7 // max selectable users (self added automatically → 8 total)

// Mode controls the behavior and hint text of the picker.
type Mode int

const (
	ModeDM           Mode = iota // Default: create DM or group
	ModeMemberPicker             // Select members for a private channel
)

// UserPickedMsg is sent when a user is selected from results.
type UserPickedMsg struct {
	User *model.User
}

// GroupPickedMsg is sent when multiple users are selected for a group channel.
type GroupPickedMsg struct {
	Users []*model.User
}

// MembersPickedMsg is sent when members are selected for a private channel.
type MembersPickedMsg struct {
	Users []*model.User
}

// SearchTriggeredMsg is sent when a search should be performed.
type SearchTriggeredMsg struct {
	Term string
}

// Model is the DM picker overlay component.
type Model struct {
	input    textinput.Model
	results  []*model.User
	selected []*model.User
	cursor   int
	visible  bool
	focused  bool
	mode     Mode
	styles   styles.Styles
	width    int
	height   int
}

// New creates a new DM picker model.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Search users..."
	ti.CharLimit = 256
	return Model{
		input:  ti,
		styles: s,
	}
}

// Open shows the DM picker overlay in DM mode.
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.mode = ModeDM
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.results = nil
	m.selected = nil
}

// OpenForMembers shows the picker in member-selection mode.
func (m *Model) OpenForMembers() {
	m.visible = true
	m.focused = true
	m.mode = ModeMemberPicker
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.results = nil
	m.selected = nil
}

// Close hides the DM picker overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	m.results = nil
	m.selected = nil
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// GetMode returns the current mode.
func (m Model) GetMode() Mode { return m.mode }

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

// toggleSelected adds or removes a user from the selected list.
func (m *Model) toggleSelected(user *model.User) {
	for i, u := range m.selected {
		if u.ID == user.ID {
			m.selected = append(m.selected[:i], m.selected[i+1:]...)
			return
		}
	}
	if len(m.selected) < maxGroupSelection {
		m.selected = append(m.selected, user)
	}
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
			return m, nil
		case tea.KeyTab:
			if len(m.results) > 0 && m.cursor < len(m.results) {
				m.toggleSelected(m.results[m.cursor])
			}
			return m, nil
		case tea.KeyEnter:
			if m.mode == ModeMemberPicker {
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
				term := m.input.Value()
				if term != "" {
					return m, func() tea.Msg { return SearchTriggeredMsg{Term: term} }
				}
				return m, nil
			}
			if len(m.selected) >= 2 {
				users := make([]*model.User, len(m.selected))
				copy(users, m.selected)
				m.Close()
				return m, func() tea.Msg { return GroupPickedMsg{Users: users} }
			}
			if len(m.results) > 0 && m.cursor < len(m.results) {
				user := m.results[m.cursor]
				m.Close()
				return m, func() tea.Msg { return UserPickedMsg{User: user} }
			}
			term := m.input.Value()
			if term != "" {
				return m, func() tea.Msg { return SearchTriggeredMsg{Term: term} }
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

// View renders the DM picker overlay.
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

	if m.mode == ModeMemberPicker {
		if len(m.selected) > 0 {
			items = append(items, m.styles.Timestamp.Render("  Press Enter to add members"))
		} else if len(m.results) == 0 && m.input.Value() != "" {
			items = append(items, m.styles.Timestamp.Render("  Press Enter to search"))
		}
	} else {
		if len(m.selected) >= 2 {
			items = append(items, m.styles.Timestamp.Render("  Press Enter to create group"))
		} else if len(m.results) == 0 && m.input.Value() != "" {
			items = append(items, m.styles.Timestamp.Render("  Press Enter to search"))
		}
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
			label = m.styles.SidebarActive.Render("> " + label)
		} else {
			label = m.styles.SidebarItem.Render("  " + label)
		}
		items = append(items, label)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
