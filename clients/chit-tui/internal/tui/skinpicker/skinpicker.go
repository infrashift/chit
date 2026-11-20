package skinpicker

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// SkinSelectedMsg is sent when a theme is selected.
type SkinSelectedMsg struct {
	Name string
}

// SkinPreviewMsg is sent when the cursor moves to a theme. The parent shows
// the theme and does not save it.
type SkinPreviewMsg struct {
	Name string
}

// SkinCancelledMsg is sent when Esc closes the picker. The parent goes back
// to the theme that was active before the previews.
type SkinCancelledMsg struct{}

// Model is the skin picker overlay component.
type Model struct {
	skins   []string
	cursor  int
	visible bool
	focused bool
	styles  styles.Styles
	width   int
	height  int
}

// New creates a new skin picker model.
func New(s styles.Styles) Model {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "skinpicker.New: styles not built by styles.New")
	return Model{styles: s}
}

// SetSkins sets the available theme names.
func (m *Model) SetSkins(names []string) {
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		assert.That(n != "", "skinpicker.SetSkins: empty theme name")
		seen[n] = true
	}
	assert.That(len(seen) == len(names), "skinpicker.SetSkins: theme listed twice")
	m.skins = names
	m.cursor = 0
}

// Open shows the skin picker overlay with the cursor on current, or on the
// first theme when the list does not hold current.
func (m *Model) Open(current string) {
	m.visible = true
	m.focused = true
	m.cursor = 0
	for i, n := range m.skins {
		if n == current {
			m.cursor = i
		}
	}
	assert.That(len(m.skins) == 0 || m.cursor < len(m.skins), "skinpicker.Open: cursor past the list")
}

// Close hides the skin picker overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() { m.focused = true }

// Blur removes focus.
func (m *Model) Blur() { m.focused = false }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "skinpicker.SetStyles: styles not built by styles.New")
	m.styles = s
}

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
	assert.That(w >= 0, "skinpicker.SetSize: negative width")
	assert.That(h >= 0, "skinpicker.SetSize: negative height")
	m.width = w
	m.height = h
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	assert.That(m.cursor >= 0, "skinpicker.Update: negative cursor")
	assert.That(len(m.skins) == 0 || m.cursor < len(m.skins), "skinpicker.Update: cursor past the list")

	switch {
	case keyMsg.Type == tea.KeyEscape:
		m.Close()
		return m, func() tea.Msg { return SkinCancelledMsg{} }
	case keyMsg.Type == tea.KeyEnter:
		if len(m.skins) > 0 && m.cursor < len(m.skins) {
			name := m.skins[m.cursor]
			assert.That(name != "", "skinpicker.Update: selected an empty name")
			m.Close()
			assert.That(!m.visible && !m.focused, "skinpicker.Update: picker still open after a choice")
			return m, func() tea.Msg { return SkinSelectedMsg{Name: name} }
		}
		return m, nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
		return m.move(-1)
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
		return m.move(1)
	}

	return m, nil
}

// move moves the cursor by delta within the list. A move to another theme
// sends SkinPreviewMsg for it.
func (m Model) move(delta int) (Model, tea.Cmd) {
	assert.That(delta == -1 || delta == 1, "skinpicker.move: a move of more than one row")
	next := min(max(m.cursor+delta, 0), max(len(m.skins)-1, 0))
	if next == m.cursor || len(m.skins) == 0 {
		return m, nil
	}
	m.cursor = next
	name := m.skins[next]
	assert.That(name != "", "skinpicker.move: an empty theme name")
	return m, func() tea.Msg { return SkinPreviewMsg{Name: name} }
}

// View renders the skin picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	assert.That(m.width >= 0, "skinpicker.View: negative width")
	assert.That(m.height >= 0, "skinpicker.View: negative height")
	var items []string
	items = append(items, m.styles.ListItemActive.Render("Select Theme"))
	items = append(items, m.styles.Timestamp.Render("  ↑/↓ preview · Enter keep · Esc undo"))

	start, end := listwin.Window(m.cursor, len(m.skins), listwin.Rows(m.height))
	assert.That(start >= 0, "skinpicker.View: window starts before the list")
	assert.That(end <= len(m.skins), "skinpicker.View: window ends past the list")
	assert.That(len(m.skins) == 0 || (m.cursor >= start && m.cursor < end), "skinpicker.View: cursor row not drawn")
	for i := start; i < end; i++ {
		assert.That(i >= 0 && i < len(m.skins), "skinpicker.View: row index outside the list")
		name := m.skins[i]
		if i == m.cursor {
			items = append(items, m.styles.ListItemActive.Render("> "+name))
		} else {
			items = append(items, m.styles.ListItem.Render("  "+name))
		}
	}

	assert.That(len(items) == 2+end-start, "skinpicker.View: row count differs from the window")
	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
