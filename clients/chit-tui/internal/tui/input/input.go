package input

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// SendMsg is sent when the user presses Enter to send a message.
type SendMsg struct {
	Content string
}

// SlashTriggerMsg is sent when the user types a slash command.
type SlashTriggerMsg struct {
	Input string
}

// AtTriggerMsg is sent when the user types an @ mention prefix.
type AtTriggerMsg struct {
	Prefix   string
	StartCol int
}

// AtDismissMsg is sent when an active @ trigger is no longer valid.
type AtDismissMsg struct{}

// Model is the input component.
type Model struct {
	textarea     textarea.Model
	focused      bool
	styles       styles.Styles
	width        int
	height       int
	prevAtActive bool
}

// New creates a new input model.
func New(s styles.Styles) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message… (Alt+Enter for new line)"
	ta.CharLimit = 65535
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	m := Model{
		textarea: ta,
		styles:   s,
	}
	assert.That(m.focused == m.textarea.Focused(), "input.New: focus flag and textarea disagree")
	assert.That(m.textarea.Height() >= 1, "input.New: textarea has no rows")
	return m
}

// Focus sets focus state and focuses the textarea.
func (m *Model) Focus() tea.Cmd {
	m.focused = true
	cmd := m.textarea.Focus()
	assert.That(m.textarea.Focused(), "Model.Focus: textarea not focused")
	return cmd
}

// Blur removes focus and blurs the textarea.
func (m *Model) Blur() {
	m.focused = false
	m.textarea.Blur()
	assert.That(!m.textarea.Focused(), "Model.Blur: textarea kept focus")
}

// Focused returns the focus state.
func (m Model) Focused() bool {
	assert.That(m.focused == m.textarea.Focused(), "Model.Focused: focus flag and textarea disagree")
	return m.focused
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets the input area dimensions.
func (m *Model) SetSize(w, h int) {
	assert.That(h > 0, "Model.SetSize: no height")
	m.width = w
	m.height = h
	m.textarea.SetWidth(w - 4)
	m.textarea.SetHeight(max(h-2, 1))
	assert.That(m.textarea.Height() >= 1, "Model.SetSize: textarea has no rows")
	assert.That(m.textarea.Height() <= max(h-2, 1), "Model.SetSize: textarea taller than the area")
}

// Value returns the current input text.
func (m Model) Value() string {
	v := m.textarea.Value()
	assert.That(utf8.RuneCountInString(v) <= m.textarea.CharLimit, "Model.Value: text over the character limit")
	return v
}

// Reset clears the input.
func (m *Model) Reset() {
	m.textarea.Reset()
	assert.That(m.textarea.Value() == "", "Model.Reset: text left behind")
	assert.That(m.textarea.Line() == 0, "Model.Reset: cursor not on the first line")
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	assert.That(m.textarea.Focused(), "Model.Update: focused input with a blurred textarea")

	// A blink changes only the cursor. It must not run the @ detection, or
	// each blink sends the mention trigger again (ISSUE-013).
	if blink, ok := msg.(cursor.BlinkMsg); ok {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(blink)
		return m, cmd
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			if keyMsg.Alt {
				// Alt+Enter: insert newline
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\n'}})
				assert.That(m.textarea.Line() < m.textarea.LineCount(), "Model.Update: cursor past the last line after a newline")
				return m, cmd
			}
			content := strings.TrimSpace(m.textarea.Value())
			if content == "" {
				return m, nil
			}
			// Check for slash command
			if strings.HasPrefix(content, "/") {
				m.textarea.Reset()
				assert.That(m.textarea.Value() == "", "Model.Update: slash command left in the input")
				return m, func() tea.Msg { return SlashTriggerMsg{Input: content} }
			}
			m.textarea.Reset()
			assert.That(m.textarea.Value() == "", "Model.Update: sent message left in the input")
			return m, func() tea.Msg { return SendMsg{Content: content} }
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	assert.That(utf8.RuneCountInString(m.textarea.Value()) <= m.textarea.CharLimit, "Model.Update: text over the character limit")
	assert.That(m.textarea.Line() < m.textarea.LineCount(), "Model.Update: cursor past the last line")

	// Detect @ mention trigger
	atCmd := m.detectAtTrigger()
	if atCmd != nil {
		return m, tea.Batch(cmd, atCmd)
	}
	return m, cmd
}

func (m *Model) detectAtTrigger() tea.Cmd {
	val := m.textarea.Value()
	lines := strings.Split(val, "\n")
	row := m.textarea.Line()
	if row < 0 || row >= len(lines) {
		if m.prevAtActive {
			m.prevAtActive = false
			return func() tea.Msg { return AtDismissMsg{} }
		}
		return nil
	}
	currentLine := lines[row]
	col := m.cursorCol()
	assert.That(col <= utf8.RuneCountInString(currentLine), "Model.detectAtTrigger: cursor past the end of its line")
	prefix, startCol, active := mention.ExtractMentionPrefix(currentLine, col)

	if active {
		assert.That(startCol >= 0 && startCol < col, "Model.detectAtTrigger: @ not before the cursor")
		assert.That(utf8.RuneCountInString(prefix) == col-startCol-1, "Model.detectAtTrigger: prefix not the text after the @")
		m.prevAtActive = true
		return func() tea.Msg { return AtTriggerMsg{Prefix: prefix, StartCol: startCol} }
	}
	if m.prevAtActive {
		m.prevAtActive = false
		return func() tea.Msg { return AtDismissMsg{} }
	}
	return nil
}

// ReplaceAtMention replaces text from @ at startCol through the current cursor
// position with @username followed by a trailing space. startCol counts
// characters. The typed prefix is deleted at the cursor and the name
// inserted there, so the cursor ends up after it on whichever line it is.
//
// It returns the command that keeps the cursor blinking. Each backspace
// starts a new blink cycle and makes the pending blink stale, so the caller
// must run the command of the last one (ISSUE-013). It is nil when nothing
// was deleted, and the old cycle goes on.
func (m *Model) ReplaceAtMention(startCol int, username string) tea.Cmd {
	assert.That(startCol >= 0, "Model.ReplaceAtMention: negative start column")
	var restartBlink tea.Cmd
	before := m.cursorCol()
	for range max(before-startCol, 0) {
		m.textarea, restartBlink = m.textarea.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	assert.That(m.cursorCol() == min(before, startCol), "Model.ReplaceAtMention: prefix not deleted back to the @")
	m.textarea.InsertString("@" + username + " ")
	assert.That(utf8.RuneCountInString(m.textarea.Value()) <= m.textarea.CharLimit, "Model.ReplaceAtMention: text over the character limit")
	assert.That(m.textarea.Line() < m.textarea.LineCount(), "Model.ReplaceAtMention: cursor past the last line")
	return restartBlink
}

// cursorCol is the cursor's position in its line, in characters.
// LineInfo's CharOffset is a display width within the wrapped row, so it
// is wrong after wide characters and on a wrapped line.
func (m Model) cursorCol() int {
	li := m.textarea.LineInfo()
	assert.That(li.StartColumn >= 0, "Model.cursorCol: negative row start")
	assert.That(li.ColumnOffset >= 0, "Model.cursorCol: negative column in the row")
	return li.StartColumn + li.ColumnOffset
}

// View renders the input area.
func (m Model) View() string {
	assert.That(m.focused == m.textarea.Focused(), "Model.View: focus flag and textarea disagree")
	assert.That(m.textarea.Height() >= 1, "Model.View: textarea has no rows")
	assert.That(m.textarea.Line() < m.textarea.LineCount(), "Model.View: cursor past the last line")
	borderStyle := m.styles.Input
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}
	return borderStyle.Width(m.width - 2).Render(m.textarea.View())
}

// SetValue replaces the input contents and puts the cursor at the end, so a
// command inserted from the palette can be completed by typing.
func (m *Model) SetValue(s string) {
	m.textarea.SetValue(s)
	m.textarea.CursorEnd()
	assert.That(utf8.RuneCountInString(m.textarea.Value()) <= m.textarea.CharLimit, "Model.SetValue: text over the character limit")
	assert.That(m.textarea.Line() == m.textarea.LineCount()-1, "Model.SetValue: cursor not on the last line")
}
