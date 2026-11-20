// Package prompt is a reusable Bubble that asks for one line of text in an
// overlay: a new tag name, the alt text of a file.
//
// Open shows the overlay with a key, a title and a first value. Enter sends
// SubmittedMsg, and the overlay stays open. The parent checks the value,
// then calls Close, or calls SetError to show why it refused the value. Esc
// closes the overlay and sends CancelledMsg.
//
// The package imports no chit package other than assert and ui/styles.
package prompt

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// SubmittedMsg carries the value of the prompt when the user pushes Enter.
type SubmittedMsg struct {
	Key   string
	Value string
}

// CancelledMsg tells that the user closed the prompt with Esc.
type CancelledMsg struct {
	Key string
}

// Model is the one-line prompt overlay.
type Model struct {
	key     string
	title   string
	hint    string
	errText string
	input   textinput.Model
	visible bool
	width   int
	height  int
	styles  styles.Styles
}

// New makes a closed prompt that accepts at most limit characters.
func New(s styles.Styles, limit int) Model {
	assert.That(limit > 0, "prompt.New: the limit must be positive")
	ti := textinput.New()
	s.ThemeTextInput(&ti)
	ti.CharLimit = limit
	ti.Prompt = "> "
	m := Model{input: ti, styles: s}
	assert.That(!m.visible, "prompt.New: a new prompt is open")
	return m
}

// Open shows the prompt for key, with title, the first value and a hint
// line. It returns the command that starts the cursor blink.
func (m *Model) Open(key, title, value, hint string) tea.Cmd {
	assert.That(key != "" && title != "", "prompt.Open: empty key or title")
	m.key, m.title, m.hint, m.errText = key, title, hint, ""
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.visible = true
	cmd := m.input.Focus()
	assert.That(m.input.Focused(), "prompt.Open: the input has no focus")
	return cmd
}

// Close hides the prompt.
func (m *Model) Close() {
	m.visible, m.errText = false, ""
	m.input.Blur()
	assert.That(!m.input.Focused(), "prompt.Close: the input kept focus")
}

// SetError shows why the parent refused the value. The prompt stays open.
func (m *Model) SetError(text string) {
	assert.That(m.visible, "prompt.SetError: the prompt is closed")
	m.errText = text
}

// Key returns the key of the open prompt.
func (m Model) Key() string { return m.key }

// Value returns the text in the prompt.
func (m Model) Value() string { return m.input.Value() }

// Visible reports whether the prompt is open.
func (m Model) Visible() bool { return m.visible }

// Focus gives the input the keys.
func (m *Model) Focus() { m.input.Focus() }

// Blur takes the keys from the input.
func (m *Model) Blur() { m.input.Blur() }

// SetSize sets the size of the screen that the overlay sits on.
func (m *Model) SetSize(w, h int) {
	assert.That(w >= 0 && h >= 0, "prompt.SetSize: negative size")
	m.width, m.height = w, h
	m.input.Width = max(w/2-6, 10)
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "prompt.SetStyles: styles not built by styles.New")
	m.styles = s
	s.ThemeTextInput(&m.input)
}

// Update handles Enter and Esc, and gives each other message to the input.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEnter:
			key, value := m.key, strings.TrimSpace(m.input.Value())
			return m, func() tea.Msg { return SubmittedMsg{Key: key, Value: value} }
		case tea.KeyEscape:
			key := m.key
			m.Close()
			return m, func() tea.Msg { return CancelledMsg{Key: key} }
		}
		m.errText = ""
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	assert.That(m.visible, "prompt.Update: an edit closed the prompt")
	return m, cmd
}

// View draws the overlay: the title, the input, then the error or the hint.
func (m Model) View() string {
	assert.That(m.input.CharLimit > 0, "prompt.View: input without a limit")
	if !m.visible {
		return ""
	}
	rows := []string{m.styles.ListItemActive.Render(m.title), "", m.input.View(), ""}
	switch {
	case m.errText != "":
		rows = append(rows, m.styles.ErrorText.Render(m.errText))
	case m.hint != "":
		rows = append(rows, m.styles.Timestamp.Render(m.hint))
	}
	assert.That(len(rows) >= 4, "prompt.View: rows missing")
	return m.styles.CmdPalette.Width(max(m.width/2, 20)).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}
