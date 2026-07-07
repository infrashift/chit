package mention

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// UserSelectedMsg is sent when a user is selected from the autocomplete popup.
type UserSelectedMsg struct {
	Username string
	StartCol int
}

// Model is the mention autocomplete popup component.
type Model struct {
	entries  []MentionEntry
	filtered []MentionEntry
	cursor   int
	visible  bool
	prefix   string
	startCol int
	styles   styles.Styles
	width    int
	maxItems int
}

// New creates a new mention autocomplete model.
func New(s styles.Styles) Model {
	return Model{
		styles:   s,
		maxItems: 7,
	}
}

// SetEntries sets the full list of mention entries.
func (m *Model) SetEntries(entries []MentionEntry) {
	m.entries = entries
}

// Show opens the popup with the given prefix and start column.
func (m *Model) Show(prefix string, startCol int) {
	m.visible = true
	m.prefix = prefix
	m.startCol = startCol
	m.filtered = FilterEntries(m.entries, prefix)
	m.cursor = 0
}

// Hide closes the popup.
func (m *Model) Hide() {
	m.visible = false
	m.cursor = 0
}

// Visible returns whether the popup is visible.
func (m Model) Visible() bool { return m.visible }

// UpdateFilter re-filters entries with a new prefix.
func (m *Model) UpdateFilter(prefix string) {
	m.prefix = prefix
	m.filtered = FilterEntries(m.entries, prefix)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(len(m.filtered)-1, 0)
	}
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetWidth sets the available width.
func (m *Model) SetWidth(w int) {
	m.width = w
}

// Update handles key messages for the popup.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	case tea.KeyEnter, tea.KeyTab:
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			selected := m.filtered[m.cursor]
			startCol := m.startCol
			m.visible = false
			m.cursor = 0
			return m, func() tea.Msg {
				return UserSelectedMsg{Username: selected.Username, StartCol: startCol}
			}
		}
		return m, nil
	case tea.KeyEscape:
		m.visible = false
		m.cursor = 0
		return m, nil
	}

	return m, nil
}

// View renders the autocomplete popup.
func (m Model) View() string {
	if !m.visible || len(m.filtered) == 0 {
		return ""
	}

	count := min(m.maxItems, len(m.filtered))
	var items []string
	for i := range count {
		e := m.filtered[i]
		label := fmt.Sprintf("@%s", e.Username)
		if e.DisplayName != "" {
			label += fmt.Sprintf("  %s", e.DisplayName)
		}
		if i == m.cursor {
			items = append(items, m.styles.AutocompleteActive.Render(label))
		} else {
			items = append(items, m.styles.AutocompleteItem.Render(label))
		}
	}

	content := strings.Join(items, "\n")
	return m.styles.AutocompletePanel.Render(content)
}
