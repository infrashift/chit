package skinpicker

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// SkinSelectedMsg is sent when a theme is selected.
type SkinSelectedMsg struct {
	Name string
}

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
	return Model{styles: s}
}

// SetSkins sets the available theme names.
func (m *Model) SetSkins(names []string) {
	m.skins = names
	m.cursor = 0
}

// Open shows the skin picker overlay.
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.cursor = 0
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
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
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

	switch {
	case keyMsg.Type == tea.KeyEscape:
		m.Close()
		return m, nil
	case keyMsg.Type == tea.KeyEnter:
		if len(m.skins) > 0 && m.cursor < len(m.skins) {
			name := m.skins[m.cursor]
			m.Close()
			return m, func() tea.Msg { return SkinSelectedMsg{Name: name} }
		}
		return m, nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.cursor < len(m.skins)-1 {
			m.cursor++
		}
		return m, nil
	}

	return m, nil
}

// View renders the skin picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.styles.SidebarActive.Render("Select Theme"))
	items = append(items, "")

	maxItems := min(len(m.skins), max((m.height/2)-4, 5))
	for i := range maxItems {
		name := m.skins[i]
		if i == m.cursor {
			items = append(items, m.styles.SidebarActive.Render("> "+name))
		} else {
			items = append(items, m.styles.SidebarItem.Render("  "+name))
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}
