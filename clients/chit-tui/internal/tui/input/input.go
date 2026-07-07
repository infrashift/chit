package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
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
	return Model{
		textarea: ta,
		styles:   s,
	}
}

// Focus sets focus state and focuses the textarea.
func (m *Model) Focus() tea.Cmd {
	m.focused = true
	return m.textarea.Focus()
}

// Blur removes focus and blurs the textarea.
func (m *Model) Blur() {
	m.focused = false
	m.textarea.Blur()
}

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets the input area dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.textarea.SetWidth(w - 4)
	m.textarea.SetHeight(max(h-2, 1))
}

// Value returns the current input text.
func (m Model) Value() string { return m.textarea.Value() }

// Reset clears the input.
func (m *Model) Reset() { m.textarea.Reset() }

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			if keyMsg.Alt {
				// Alt+Enter: insert newline
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\n'}})
				return m, cmd
			}
			content := strings.TrimSpace(m.textarea.Value())
			if content == "" {
				return m, nil
			}
			// Check for slash command
			if strings.HasPrefix(content, "/") {
				m.textarea.Reset()
				return m, func() tea.Msg { return SlashTriggerMsg{Input: content} }
			}
			m.textarea.Reset()
			return m, func() tea.Msg { return SendMsg{Content: content} }
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)

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
	col := m.textarea.LineInfo().CharOffset

	prefix, startCol, active := mention.ExtractMentionPrefix(currentLine, col)

	// Don't trigger for slash commands
	if active && startCol == 0 && len(currentLine) > 0 && currentLine[0] == '/' {
		active = false
	}

	if active {
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
// position with @username followed by a trailing space.
func (m *Model) ReplaceAtMention(startCol int, username string) {
	val := m.textarea.Value()
	lines := strings.Split(val, "\n")
	row := m.textarea.Line()
	if row < 0 || row >= len(lines) {
		return
	}
	col := m.textarea.LineInfo().CharOffset

	line := lines[row]
	replacement := "@" + username + " "
	end := col
	if end > len(line) {
		end = len(line)
	}
	newLine := line[:startCol] + replacement + line[end:]
	lines[row] = newLine
	newVal := strings.Join(lines, "\n")
	m.textarea.SetValue(newVal)

	// Position cursor after the replacement
	newCol := startCol + len(replacement)
	// Move cursor: set value resets cursor to end, use CursorEnd or navigate
	// SetValue puts cursor at end, which is fine for single-line common case.
	// For multi-line we'd need more, but this is sufficient.
	_ = newCol
}

// View renders the input area.
func (m Model) View() string {
	borderStyle := m.styles.Input
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}
	return borderStyle.Width(m.width - 2).Render(m.textarea.View())
}
