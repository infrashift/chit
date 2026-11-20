// Package help renders a static overlay listing keybindings and mouse
// actions. It opens with "?" (outside the input box) or the action-bar
// button, and any key closes it.
package help

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	assert.That(s.CmdPalette.GetBorderStyle().Left != "", "help.New: styles not built by styles.New")
	assert.That(s.CmdPalette.GetPaddingLeft() > 0, "help.New: styles without the overlay padding")
	return Model{styles: s}
}

// keyColumn is the width the key column is padded to.
const keyColumn = 8

// Open shows the overlay.
func (m *Model) Open() {
	assert.That(m.styles.CmdPalette.GetBorderStyle().Left != "", "Model.Open: styles not set by New")
	m.visible = true
}

// Close hides the overlay.
func (m *Model) Close() {
	assert.That(m.styles.CmdPalette.GetBorderStyle().Left != "", "Model.Close: styles not set by New")
	m.visible = false
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.CmdPalette.GetBorderStyle().Left != "", "Model.SetStyles: styles not built by styles.New")
	m.styles = s
}

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
	assert.That(m.styles.CmdPalette.GetBorderStyle().Left != "", "Model.Update: styles not set by New")
	if _, ok := msg.(tea.KeyMsg); ok {
		m.Close()
		assert.That(!m.visible, "Model.Update: overlay still open after a key")
	}
	return m, nil
}

type binding struct {
	keys string
	desc string
}

// keyBindings lists the keyboard shortcuts, in the order shown.
func keyBindings() []binding {
	bs := []binding{
		{"ctrl+k", "open palette: jump to channels and DMs"},
		{"  @ / ?", "palette modes: people, commands, message search"},
		{"  ??", "search every channel, not just this one"},
		{"ctrl+s", "search messages in the active channel"},
		{"ctrl+d", "find people / start a DM"},
		{"ctrl+n", "create a channel"},
		{"tab", "switch between panes"},
		{"enter", "history: reply in thread · input: send"},
		{"j / k", "history: move between posts"},
		{"t", "tag a post (history pane, or a thread's root); new tags are the channel's, ctrl+t in the picker for private"},
		{"y", "copy the selection, or the selected post"},
		{"e", "edit your own post (history pane)"},
		{"d d", "delete your own post, pressing d twice (history pane)"},
		{"p", "pin or unpin a post (history pane)"},
		{"esc", "close overlay / leave thread / back to history"},
		{"?", "this help (history or thread pane)"},
		{"ctrl+t", "themes: up/down shows each one, enter keeps it, esc undoes"},
		{"i", "post details: agent, on behalf of, provenance, files"},
		{"ctrl+c", "quit"},
	}
	assert.That(len(bs) > 0, "keyBindings: no bindings")
	seen := make(map[string]bool, len(bs))
	for _, b := range bs {
		assert.That(!seen[b.keys], "keyBindings: keys listed twice")
		seen[b.keys] = true
		assert.That(b.keys != "" && b.desc != "", "keyBindings: binding without keys or description")
		assert.That(len(b.keys) <= keyColumn, "keyBindings: keys wider than the key column")
	}
	return bs
}

// hints are tips shown below the bindings. Replying is the capability people
// most often miss: the history pane has to be focused before enter opens a
// thread, since enter sends the message while the input is focused.
func hints() []string {
	hs := []string{
		"To reply: press tab to focus the history pane, pick a post with j/k, then enter.",
		"A theme changes at once, with no restart. A theme file that you edit loads again by itself.",
	}
	assert.That(len(hs) > 0, "hints: no hints")
	for _, h := range hs {
		assert.That(h != "", "hints: empty hint")
	}
	return hs
}

// clientCommands are handled by chit-tui itself and never reach the server.
// Everything else typed with a leading slash is sent as a message, and the
// server decides whether it is a command.
func clientCommands() []binding {
	bs := []binding{
		{"/theme", "choose a theme; /theme <name> sets one, /theme reload reads it again (alias: /skin)"},
		{"/chtag", "channel tags: add <name>, rm <name>, find <name>…"},
		{"/attach", "attach an image to the next post (a picker when no path)"},
		{"/detach", "remove the files that wait for the next post"},
		{"/group", "start a group conversation with three or more people"},
		{"/nick", "change your display name"},
		{"/username", "change your username (breaks existing @mentions)"},
		{"/threads", "threads you follow, in this team and in DMs"},
		{"/leave", "leave the current channel"},
		{"/logout", "sign out and clear the stored session"},
	}
	assert.That(len(bs) > 0, "clientCommands: no commands")
	seen := make(map[string]bool, len(bs))
	for _, b := range bs {
		assert.That(!seen[b.keys], "clientCommands: command listed twice")
		seen[b.keys] = true
		assert.That(strings.HasPrefix(b.keys, "/"), "clientCommands: command without its slash")
		assert.That(b.desc != "", "clientCommands: command without a description")
	}
	return bs
}

// mouseBindings lists what the mouse does.
func mouseBindings() []binding {
	bs := []binding{
		{"wheel", "scroll history / move palette cursor"},
		{"click", "action-bar buttons, palette rows, pick a post"},
		{"drag", "select lines in the history; y copies them"},
	}
	assert.That(len(bs) > 0, "mouseBindings: no bindings")
	seen := make(map[string]bool, len(bs))
	for _, b := range bs {
		assert.That(!seen[b.keys], "mouseBindings: keys listed twice")
		seen[b.keys] = true
		assert.That(b.keys != "" && b.desc != "", "mouseBindings: binding without keys or description")
		assert.That(len(b.keys) <= keyColumn, "mouseBindings: keys wider than the key column")
	}
	return bs
}

// View renders the help overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	assert.That(m.styles.CmdPalette.GetBorderStyle().Left != "", "Model.View: styles not set by New")
	header := m.styles.ListItemActive.Render("Keyboard")
	items := []string{header}
	for _, b := range keyBindings() {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, "", m.styles.ListItemActive.Render("Commands"))
	for _, b := range clientCommands() {
		items = append(items, m.renderBinding(b))
	}
	items = append(items, m.renderBinding(binding{
		keys: "/…", desc: "anything else is sent to the server",
	}))

	items = append(items, "", m.styles.ListItemActive.Render("Mouse"))
	for _, b := range mouseBindings() {
		items = append(items, m.renderBinding(b))
	}

	for _, h := range hints() {
		items = append(items, "", m.styles.Timestamp.Render("  "+h))
	}
	items = append(items, "", m.styles.Timestamp.Render("press any key to close"))

	// The headings and blank lines: Keyboard, blank+Commands, the catch-all
	// row, blank+Mouse, a blank before each hint, blank+the closing line.
	want := 1 + len(keyBindings()) + 2 + len(clientCommands()) + 1 + 2 +
		len(mouseBindings()) + 2*len(hints()) + 2
	assert.That(len(items) == want, "Model.View: a row lost or added")

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	width := min(m.width/2, 64)
	assert.That(width <= 64, "Model.View: overlay wider than its cap")
	return m.styles.CmdPalette.Width(width).Render(content)
}

func (m Model) renderBinding(b binding) string {
	assert.That(b.keys != "" && b.desc != "", "Model.renderBinding: binding without keys or description")
	key := m.styles.MentionText.Render(padRight(b.keys, keyColumn))
	out := "  " + key + "  " + m.styles.ListItem.Render(b.desc)
	assert.That(len(out) >= 4+keyColumn+len(b.desc), "Model.renderBinding: row shorter than its parts")
	assert.That(strings.HasPrefix(out, "  "), "Model.renderBinding: row not indented")
	return out
}

func padRight(s string, n int) string {
	assert.That(n >= 0, "padRight: negative width")
	orig := s
	for len(s) < n {
		s += " "
	}
	assert.That(len(s) >= n, "padRight: result narrower than the width")
	assert.That(len(s) == max(n, len(orig)), "padRight: padded past the width")
	assert.That(strings.HasPrefix(s, orig), "padRight: text changed")
	return s
}
