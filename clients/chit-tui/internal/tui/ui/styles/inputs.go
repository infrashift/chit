package styles

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// setInputs sets the styles of the text boxes and the terminal colors.
func (s *Styles) setInputs(t theme.Theme) {
	assert.That(s != nil, "Styles.setInputs: nil receiver")
	base := lipgloss.NewStyle()
	s.InputText = fg(base, t.Foreground)
	s.InputPlaceholder = fg(base, t.Muted)
	s.InputPrompt = fg(base, t.Accent)
	s.InputCursor = fg(base, t.Accent)
	s.TermForeground = hexOf(t.Foreground)
	s.TermBackground = hexOf(t.Background)
	assert.That(s.TermBackground == "" || len(s.TermBackground) == 7, "Styles.setInputs: background not #rrggbb")
}

// hexOf returns c as "#rrggbb", or "" for nil.
func hexOf(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, a := c.RGBA()
	// The terminal color has no alpha. A color that is not opaque keeps
	// its premultiplied channels, which is what the cells show.
	assert.That(a <= 0xffff, "hexOf: alpha out of range")
	out := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	assert.That(len(out) == 7, "hexOf: result not #rrggbb")
	return out
}

// ThemeTextInput gives a text input the colors of the theme. Each component
// calls it in New and in SetStyles.
func (s Styles) ThemeTextInput(ti *textinput.Model) {
	assert.That(ti != nil, "Styles.ThemeTextInput: nil input")
	ti.TextStyle = s.InputText
	ti.PlaceholderStyle = s.InputPlaceholder
	ti.CompletionStyle = s.InputPlaceholder
	ti.PromptStyle = s.InputPrompt
	ti.Cursor.Style = s.InputCursor
	ti.Cursor.TextStyle = s.InputText
	assert.That(ti.PromptStyle.GetForeground() == s.InputPrompt.GetForeground(), "Styles.ThemeTextInput: prompt style not set")
}

// ThemeTextArea gives a text area the colors of the theme, focused and
// blurred. The cursor line has no background of its own.
func (s Styles) ThemeTextArea(ta *textarea.Model) {
	assert.That(ta != nil, "Styles.ThemeTextArea: nil text area")
	st := textarea.Style{
		Base:             lipgloss.NewStyle(),
		CursorLine:       s.InputText,
		CursorLineNumber: s.InputPlaceholder,
		EndOfBuffer:      s.InputPlaceholder,
		LineNumber:       s.InputPlaceholder,
		Placeholder:      s.InputPlaceholder,
		Prompt:           s.InputPrompt,
		Text:             s.InputText,
	}
	ta.FocusedStyle, ta.BlurredStyle = st, st
	ta.Cursor.Style = s.InputCursor
	ta.Cursor.TextStyle = s.InputText
	assert.That(ta.FocusedStyle.Text.GetForeground() == s.InputText.GetForeground(), "Styles.ThemeTextArea: text style not set")
}
