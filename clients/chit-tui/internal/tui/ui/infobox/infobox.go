// Package infobox is a reusable Bubble that shows the facts about one thing
// in an overlay: a title, rows of a label and a value, and the keys of the
// actions that the parent offers.
//
// Open shows the overlay for a subject. Up and down scroll a long list. A
// key of an action sends ActionMsg, and the overlay stays open. Esc or q
// closes it and sends ClosedMsg.
//
// The package imports no chit package other than assert, ui/styles and
// ui/listwin.
package infobox

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// maxRows is the most rows that the box holds.
const maxRows = 200

// Row is one fact: a label and a value. A row with no label continues the
// row above it.
type Row struct {
	Label string
	Value string
}

// Action is a key that the parent acts on, with a short label.
type Action struct {
	Key   string
	Label string
}

// ActionMsg tells that the user pushed the key of an action.
type ActionMsg struct {
	Subject string
	Key     string
}

// ClosedMsg tells that the user closed the box.
type ClosedMsg struct {
	Subject string
}

// Model is the info box overlay.
type Model struct {
	subject string
	title   string
	rows    []Row
	actions []Action
	offset  int
	visible bool
	width   int
	height  int
	styles  styles.Styles
}

// New makes a closed box.
func New(s styles.Styles) Model {
	assert.That(s.CmdPalette.GetBorderStyle().Left != "", "infobox.New: styles not built by styles.New")
	m := Model{styles: s}
	assert.That(!m.visible, "infobox.New: a new box is open")
	return m
}

// Open shows the box for subject. It keeps at most maxRows rows.
func (m *Model) Open(subject, title string, rows []Row, actions []Action) {
	assert.That(subject != "" && title != "", "infobox.Open: empty subject or title")
	for _, a := range actions {
		assert.That(len([]rune(a.Key)) == 1 && a.Key != "q", "infobox.Open: an action key must be one rune and not q")
	}
	m.subject, m.title, m.actions, m.offset, m.visible = subject, title, actions, 0, true
	m.rows = rows[:min(len(rows), maxRows)]
	assert.That(len(m.rows) <= maxRows, "infobox.Open: rows over the cap")
}

// Close hides the box.
func (m *Model) Close() {
	m.visible = false
	assert.That(!m.visible, "infobox.Close: still open")
}

// Visible reports whether the box is open.
func (m Model) Visible() bool { return m.visible }

// Subject returns the subject of the open box.
func (m Model) Subject() string { return m.subject }

// SetSize sets the size of the screen that the overlay sits on.
func (m *Model) SetSize(w, h int) {
	assert.That(w >= 0 && h >= 0, "infobox.SetSize: negative size")
	m.width, m.height = w, h
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.CmdPalette.GetBorderStyle().Left != "", "infobox.SetStyles: styles not built by styles.New")
	m.styles = s
}

// Update scrolls, sends the action of a key, or closes on Esc or q.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !m.visible || !ok {
		return m, nil
	}
	subject := m.subject
	switch k.String() {
	case "esc", "q":
		m.Close()
		return m, func() tea.Msg { return ClosedMsg{Subject: subject} }
	case "up", "k":
		m.offset = max(m.offset-1, 0)
		return m, nil
	case "down", "j":
		m.offset = min(m.offset+1, max(len(m.rows)-m.visibleRows(), 0))
		return m, nil
	}
	for _, a := range m.actions {
		if k.String() == a.Key {
			key := a.Key
			return m, func() tea.Msg { return ActionMsg{Subject: subject, Key: key} }
		}
	}
	assert.That(m.offset >= 0, "infobox.Update: negative offset")
	return m, nil
}

func (m Model) visibleRows() int { return listwin.Rows(m.height) }

// View draws the title, the rows that fit and the key hints.
func (m Model) View() string {
	if !m.visible {
		return ""
	}
	width := min(max(m.width*2/3, 40), 100)
	inner := width - 6
	labelW := 0
	for _, r := range m.rows {
		labelW = max(labelW, ansi.StringWidth(r.Label))
	}
	out := []string{m.styles.ListItemActive.Render(m.title), ""}
	end := min(m.offset+m.visibleRows(), len(m.rows))
	for _, r := range m.rows[m.offset:end] {
		label := m.styles.Timestamp.Render(r.Label + strings.Repeat(" ", labelW-ansi.StringWidth(r.Label)))
		value := ansi.Truncate(r.Value, max(inner-labelW-2, 8), "…")
		out = append(out, label+"  "+m.styles.ListItem.Render(value))
	}
	out = append(out, "", m.styles.Timestamp.Render(m.hints(end)))
	assert.That(len(out) == 4+end-m.offset, "infobox.View: a row lost or added")
	return m.styles.CmdPalette.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, out...))
}

// hints names the keys: the actions, the scroll when there is more, and Esc.
func (m Model) hints(end int) string {
	var parts []string
	for _, a := range m.actions {
		if a.Label != "" {
			parts = append(parts, a.Key+" "+a.Label)
		}
	}
	if m.offset > 0 || end < len(m.rows) {
		parts = append(parts, "↑/↓ scroll")
	}
	parts = append(parts, "esc close")
	assert.That(len(parts) >= 1, "infobox.hints: no hint")
	return strings.Join(parts, " · ")
}
