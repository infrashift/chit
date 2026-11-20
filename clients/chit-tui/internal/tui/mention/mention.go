package mention

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	m := Model{
		styles:   s,
		maxItems: 7,
	}
	assert.That(m.maxItems > 0, "mention.New: popup shows no rows")
	return m
}

// SetEntries sets the full list of mention entries.
func (m *Model) SetEntries(entries []MentionEntry) {
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.SetEntries: cursor outside the list")
	m.entries = entries
}

// Show opens the popup with the given prefix and start column.
func (m *Model) Show(prefix string, startCol int) {
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.Show: cursor outside the list")
	m.visible = true
	m.prefix = prefix
	assert.That(startCol >= 0, "Model.Show: negative start column")
	m.startCol = startCol
	m.filtered = FilterEntries(m.entries, prefix)
	m.cursor = 0
	assert.That(len(m.filtered) <= len(m.entries), "Model.Show: more matches than entries")
}

// Hide closes the popup.
func (m *Model) Hide() {
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.Hide: cursor outside the list")
	m.visible = false
	m.cursor = 0
}

// Visible returns whether the popup is visible.
func (m Model) Visible() bool { return m.visible }

// UpdateFilter re-filters entries with a new prefix.
func (m *Model) UpdateFilter(prefix string) {
	assert.That(m.startCol >= 0, "Model.UpdateFilter: negative start column")
	m.prefix = prefix
	m.filtered = FilterEntries(m.entries, prefix)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(len(m.filtered)-1, 0)
	}
	assert.That(len(m.filtered) <= len(m.entries), "Model.UpdateFilter: more matches than entries")
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.UpdateFilter: cursor outside the list")
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetWidth sets the available width.
func (m *Model) SetWidth(w int) {
	assert.That(w >= 0, "Model.SetWidth: negative width")
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
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.Update: cursor outside the list")

	switch keyMsg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		assert.That(m.cursor >= 0, "Model.Update: cursor above the list")
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		assert.That(m.cursor <= max(len(m.filtered)-1, 0), "Model.Update: cursor below the list")
		return m, nil
	case tea.KeyEnter, tea.KeyTab:
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			selected := m.filtered[m.cursor]
			startCol := m.startCol
			assert.That(startCol >= 0, "Model.Update: negative start column")
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

	assert.That(m.maxItems > 0, "Model.View: popup shows no rows")
	assert.That(m.cursor >= 0 && m.cursor < len(m.filtered), "Model.View: cursor outside the list")
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

	assert.That(len(items) == count, "Model.View: row count does not match the matches shown")
	content := strings.Join(items, "\n")
	return m.styles.AutocompletePanel.Render(content)
}
