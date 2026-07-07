// Package help renders a static overlay listing keybindings and mouse
// actions. It opens with "?" (outside the input box) or the action-bar
// button, and any key closes it.
package help

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Model is the help overlay component.
type Model struct {
	visible bool
	styles  styles.Styles
	width   int
	height  int
}

// New creates a new help overlay.
func New(s styles.Styles) Model {
	return Model{styles: s}
}

// Open shows the overlay.
func (m *Model) Open() { m.visible = true }

// Close hides the overlay.
func (m *Model) Close() { m.visible = false }

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets the available screen dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update closes the overlay on any key press.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	if _, ok := msg.(tea.KeyMsg); ok {
		m.Close()
	}
	return m, nil
}

type binding struct {
	keys string
	desc string
}

var keyBindings = []binding{
	{"ctrl+k", "open palette: jump to channels and DMs"},
	{"  @ / ?", "palette modes: people, commands, message search"},
	{"ctrl+s", "search messages in the active channel"},
	{"ctrl+d", "find people / start a DM"},
	{"ctrl+n", "create a channel"},
	{"tab", "switch between panes"},
	{"enter", "history: open thread · input: send"},
	{"t", "tag the selected post (history pane)"},
	{"esc", "close overlay / leave thread / back to history"},
	{"?", "this help (history pane)"},
	{"ctrl+c", "quit"},
}

var mouseBindings = []binding{
	{"wheel", "scroll history / move palette cursor"},
	{"click", "action-bar buttons and palette rows"},
}

// View renders the help overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	header := m.styles.SidebarActive.Render("Keyboard")
	items := []string{header}
	for _, b := range keyBindings {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, "", m.styles.SidebarActive.Render("Mouse"))
	for _, b := range mouseBindings {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, "", m.styles.Timestamp.Render("press any key to close"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(min(m.width/2, 64)).Render(content)
}

func (m Model) renderBinding(b binding) string {
	key := m.styles.MentionText.Render(padRight(b.keys, 8))
	return "  " + key + "  " + m.styles.SidebarItem.Render(b.desc)
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}
