// Package filepick is a reusable Bubble that picks one file in an overlay.
// It wraps the file picker of bubbles, and adds a list of file extensions,
// a size cap and the overlay frame.
//
// Open shows the overlay in a directory and returns the command that reads
// it. The parent gives each message to Update while the overlay is open.
// A pick of an allowed file of the right size closes the overlay and sends
// PickedMsg. Another pick shows the reason and keeps the overlay open. Esc
// closes it and sends CancelledMsg.
//
// The package imports no chit package other than assert and ui/styles.
package filepick

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// PickedMsg carries the path and the size of the picked file.
type PickedMsg struct {
	Path string
	Size int64
}

// CancelledMsg tells that the user closed the overlay with Esc.
type CancelledMsg struct{}

// Model is the file picker overlay.
type Model struct {
	picker   filepicker.Model
	title    string
	exts     []string
	maxBytes int64
	errText  string
	visible  bool
	width    int
	height   int
	styles   styles.Styles
}

// New makes a closed picker for files with one of exts (each with its dot)
// and of at most maxBytes.
func New(s styles.Styles, title string, exts []string, maxBytes int64) Model {
	assert.That(title != "" && len(exts) > 0, "filepick.New: empty title or extension list")
	assert.That(maxBytes > 0, "filepick.New: the size cap must be positive")
	fp := filepicker.New()
	fp.AllowedTypes = exts
	fp.ShowPermissions = false
	fp.AutoHeight = false
	m := Model{picker: fp, title: title, exts: exts, maxBytes: maxBytes, styles: s}
	m.applyStyles()
	return m
}

// Open shows the picker in dir and returns the command that reads dir.
func (m *Model) Open(dir string) tea.Cmd {
	assert.That(dir != "", "filepick.Open: empty directory")
	m.picker.CurrentDirectory = dir
	m.picker.Path, m.errText, m.visible = "", "", true
	m.SetSize(m.width, m.height)
	assert.That(m.visible, "filepick.Open: not open")
	return m.picker.Init()
}

// Close hides the picker.
func (m *Model) Close() {
	m.visible, m.errText = false, ""
	assert.That(!m.visible, "filepick.Close: still open")
}

// Visible reports whether the picker is open.
func (m Model) Visible() bool { return m.visible }

// SetSize sets the size of the screen that the overlay sits on.
func (m *Model) SetSize(w, h int) {
	assert.That(w >= 0 && h >= 0, "filepick.SetSize: negative size")
	m.width, m.height = w, h
	m.picker.SetHeight(max(h/2-6, 3))
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "filepick.SetStyles: styles not built by styles.New")
	m.styles = s
	m.applyStyles()
}

// applyStyles maps the chit styles onto the styles of the inner picker.
func (m *Model) applyStyles() {
	st := filepicker.DefaultStyles()
	st.Cursor = m.styles.ListItemActive
	st.Selected = m.styles.ListItemActive
	st.Directory = m.styles.Username
	st.File = m.styles.ListItem
	st.DisabledFile = m.styles.Timestamp
	st.DisabledCursor = m.styles.Timestamp
	st.DisabledSelected = m.styles.Timestamp
	st.FileSize = m.styles.Timestamp
	st.EmptyDirectory = m.styles.Timestamp
	m.picker.Styles = st
	assert.That(m.picker.Styles.Cursor.GetForeground() == m.styles.ListItemActive.GetForeground(), "filepick.applyStyles: cursor style not set")
}

// Update handles Esc and the picks, and gives each other message to the
// inner picker.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEscape {
		m.Close()
		return m, func() tea.Msg { return CancelledMsg{} }
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if ok, path := m.picker.DidSelectFile(msg); ok {
		return m.pick(path, cmd)
	}
	if ok, path := m.picker.DidSelectDisabledFile(msg); ok {
		m.errText = fmt.Sprintf("%s: pick a file of type %s", baseName(path), strings.Join(m.exts, ", "))
	}
	return m, cmd
}

// pick checks the size of path, then closes and sends PickedMsg, or shows
// the reason and stays open.
func (m Model) pick(path string, cmd tea.Cmd) (Model, tea.Cmd) {
	assert.That(path != "", "filepick.pick: empty path")
	info, err := os.Stat(path)
	switch {
	case err != nil:
		m.errText = err.Error()
		return m, cmd
	case info.Size() > m.maxBytes:
		m.errText = fmt.Sprintf("%s is too large (%d MB at most)", baseName(path), m.maxBytes>>20)
		return m, cmd
	}
	m.Close()
	picked := PickedMsg{Path: path, Size: info.Size()}
	assert.That(picked.Size <= m.maxBytes, "filepick.pick: picked a file over the cap")
	return m, tea.Batch(cmd, func() tea.Msg { return picked })
}

// View draws the overlay: the title, the directory, the list, then the
// error or the key hint.
func (m Model) View() string {
	if !m.visible {
		return ""
	}
	foot := m.styles.Timestamp.Render("↑/↓ move · → or Enter open · ← back · Esc close")
	if m.errText != "" {
		foot = m.styles.ErrorText.Render(m.errText)
	}
	width := max(m.width/2, 30)
	rows := []string{
		m.styles.ListItemActive.Render(m.title),
		m.styles.Timestamp.Render(tailOf(m.picker.CurrentDirectory, width-6)),
		"", m.picker.View(), foot,
	}
	assert.That(len(rows) == 5, "filepick.View: rows missing")
	return m.styles.CmdPalette.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// tailOf returns s, or "…" and the end of s when s is wider than w cells.
// The end of a path names the directory, so it is the part to keep.
func tailOf(s string, w int) string {
	if w < 2 || ansi.StringWidth(s) <= w {
		return s
	}
	out := "…" + ansi.TruncateLeft(s, ansi.StringWidth(s)-w+1, "")
	assert.That(ansi.StringWidth(out) <= w, "filepick.tailOf: result wider than w")
	return out
}

func baseName(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	return path[i+1:]
}
