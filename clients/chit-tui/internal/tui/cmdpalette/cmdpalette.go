package cmdpalette

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit-tui/internal/model"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
)

// CommandSelectedMsg is sent when a command is selected from the palette.
type CommandSelectedMsg struct {
	Command *model.Command
}

// Model is the command palette overlay.
type Model struct {
	input    textinput.Model
	commands []*model.Command
	filtered []*model.Command
	cursor   int
	visible  bool
	focused  bool
	styles   styles.Styles
	width    int
	height   int
}

// New creates a new command palette.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Search commands..."
	ti.CharLimit = 128
	return Model{
		input:  ti,
		styles: s,
	}
}

// SetCommands sets the available commands.
func (m *Model) SetCommands(cmds []*model.Command) {
	m.commands = cmds
	m.filtered = cmds
}

// Open shows the palette.
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.filtered = m.commands
}

// Close hides the palette.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

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

// SetSize sets the palette dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w - 8
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
			if m.cursor < len(m.filtered) {
				cmd := m.filtered[m.cursor]
				m.Close()
				return m, func() tea.Msg { return CommandSelectedMsg{Command: cmd} }
			}
			return m, nil
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
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filter()
	return m, cmd
}

func (m *Model) filter() {
	query := strings.ToLower(m.input.Value())
	if query == "" {
		m.filtered = m.commands
		m.cursor = 0
		return
	}

	var filtered []*model.Command
	for _, c := range m.commands {
		if strings.Contains(strings.ToLower(c.Slug), query) ||
			strings.Contains(strings.ToLower(c.Description), query) {
			filtered = append(filtered, c)
		}
	}
	m.filtered = filtered
	m.cursor = 0
}

// View renders the command palette.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.input.View())
	items = append(items, "")

	maxItems := min(len(m.filtered), max((m.height/2)-4, 5))
	for i := range maxItems {
		c := m.filtered[i]
		line := "/" + c.Slug + "  " + m.styles.Timestamp.Render(c.Description)
		if i == m.cursor {
			line = m.styles.SidebarActive.Render("> " + line)
		} else {
			line = m.styles.SidebarItem.Render("  " + line)
		}
		items = append(items, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(content)
}
