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
	{"  ??", "search every channel, not just this one"},
	{"ctrl+s", "search messages in the active channel"},
	{"ctrl+d", "find people / start a DM"},
	{"ctrl+n", "create a channel"},
	{"tab", "switch between panes"},
	{"enter", "history: reply in thread · input: send"},
	{"j / k", "history: move between posts"},
	{"t", "tag a post (history pane, or a thread's root)"},
	{"y", "copy the selection, or the selected post"},
	{"e", "edit your own post (history pane)"},
	{"d d", "delete your own post, pressing d twice (history pane)"},
	{"p", "pin or unpin a post (history pane)"},
	{"esc", "close overlay / leave thread / back to history"},
	{"?", "this help (history pane)"},
	{"ctrl+c", "quit"},
}

// Replying is the capability people most often miss: the history pane has to
// be focused before enter opens a thread, since enter sends the message while
// the input is focused.
var hints = []string{
	"To reply: press tab to focus the history pane, pick a post with j/k, then enter.",
}

// clientCommands are handled by chit-tui itself and never reach the server.
// Everything else typed with a leading slash is sent as a message, and the
// server decides whether it is a command.
var clientCommands = []binding{
	{"/theme", "choose a theme (alias: /skin)"},
	{"/group", "start a group conversation with three or more people"},
	{"/nick", "change your display name"},
	{"/username", "change your username (breaks existing @mentions)"},
	{"/threads", "threads you follow in this team"},
	{"/leave", "leave the current channel"},
	{"/logout", "sign out and clear the stored session"},
}

var mouseBindings = []binding{
	{"wheel", "scroll history / move palette cursor"},
	{"click", "action-bar buttons, palette rows, pick a post"},
	{"drag", "select lines in the history; y copies them"},
}

// View renders the help overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	header := m.styles.ListItemActive.Render("Keyboard")
	items := []string{header}
	for _, b := range keyBindings {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, "", m.styles.ListItemActive.Render("Commands"))
	for _, b := range clientCommands {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, m.renderBinding(binding{
		keys: "/…", desc: "anything else is sent to the server",
	}))

	items = append(items, "", m.styles.ListItemActive.Render("Mouse"))
	for _, b := range mouseBindings {
		items = append(items, m.renderBinding(b))
	}

	for _, h := range hints {
		items = append(items, "", m.styles.Timestamp.Render("  "+h))
	}
	items = append(items, "", m.styles.Timestamp.Render("press any key to close"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(min(m.width/2, 64)).Render(content)
}

func (m Model) renderBinding(b binding) string {
	key := m.styles.MentionText.Render(padRight(b.keys, 8))
	return "  " + key + "  " + m.styles.ListItem.Render(b.desc)
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}
