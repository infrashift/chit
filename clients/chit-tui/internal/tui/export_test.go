package tui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// SetClipboardOutput redirects what copying writes, so tests that copy do
// not print escape sequences into the test output.
func (m *Model) SetClipboardOutput(w io.Writer) { m.clipboardOut = w }

// Focused reports which area has keyboard focus. Where keys land decides
// what single-letter keys do, so tests need to see it directly.
func (m Model) Focused() FocusArea { return m.focus }

// SelectedPostID returns the ID of the post under the history cursor, or "".
// Exported for tests, which cannot reach the viewport's cursor otherwise.
func (m Model) SelectedPostID() string {
	if p := m.viewport.SelectedPost(); p != nil {
		return p.ID
	}
	return ""
}

// HasSelection reports whether the history pane holds a selection. Exported
// for tests, which cannot reach the viewport otherwise.
func (m Model) HasSelection() bool { return m.viewport.HasSelection() }

// ThemeWatchTick returns the next tick of the live theme-file watch loop of
// m, so a test need not wait for the timer.
func ThemeWatchTick(m Model) tea.Msg { return themeWatchMsg{seq: m.watchSeq} }
