package search

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// ResultSelectedMsg is sent when a search result is selected.
type ResultSelectedMsg struct {
	Post *model.Post
}

// SubmitMsg is sent when the user submits a search query.
type SubmitMsg struct {
	Term string
}

// Model is the search overlay component.
type Model struct {
	input     textinput.Model
	results   []*model.Post
	cursor    int
	visible   bool
	focused   bool
	submitted bool
	err       string
	styles    styles.Styles
	width     int
	height    int
}

// New creates a new search model.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Search posts..."
	ti.CharLimit = 256
	return Model{
		input:  ti,
		styles: s,
	}
}

// Open shows the search overlay.
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.submitted = false
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.results = nil
	m.err = ""
}

// Close hides the search overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	m.results = nil
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

// SetResults sets the search results.
func (m *Model) SetResults(posts []*model.Post) {
	m.results = posts
	m.cursor = 0
	m.err = ""
}

// SetError sets an error message to display in the overlay.
func (m *Model) SetError(errText string) {
	m.err = errText
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
		case tea.KeyEnter:
			if len(m.results) > 0 && m.cursor < len(m.results) {
				post := m.results[m.cursor]
				m.Close()
				return m, func() tea.Msg { return ResultSelectedMsg{Post: post} }
			}
			// No results yet — submit search
			term := m.input.Value()
			if term != "" {
				m.submitted = true
				return m, func() tea.Msg { return SubmitMsg{Term: term} }
			}
			return m, nil
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

	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.submitted = false
	}
	return m, cmd
}

// View renders the search overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.input.View())
	items = append(items, "")

	if m.err != "" {
		items = append(items, m.styles.ErrorText.Render("  "+m.err))
	} else if len(m.results) == 0 && m.input.Value() != "" {
		if m.submitted {
			items = append(items, m.styles.Timestamp.Render("  No matches found"))
		} else {
			items = append(items, m.styles.Timestamp.Render("  Press Enter to search"))
		}
	}

	maxItems := min(len(m.results), max((m.height/2)-4, 5))
	for i := range maxItems {
		p := m.results[i]
		content := p.Content
		if len(content) > 60 {
			content = content[:60] + "..."
		}
		line := fmt.Sprintf("%s: %s", p.UserID, content)
		if i == m.cursor {
			line = m.styles.SidebarActive.Render("> " + line)
		} else {
			line = m.styles.SidebarItem.Render("  " + line)
		}
		items = append(items, line)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
