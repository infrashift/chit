package tagpicker

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// TagToggledMsg is sent when a tag is toggled on/off for a post.
type TagToggledMsg struct {
	PostID  string
	TagID   string
	TagName string
	Applied bool
}

// TagCreateRequestMsg is sent when the user wants to create a new tag. It
// is a channel tag of the post's channel unless Private is set.
type TagCreateRequestMsg struct {
	Name    string
	PostID  string
	Private bool
}

// Model is the tag picker overlay component.
type Model struct {
	input    textinput.Model
	allTags  []*model.Tag
	postTags map[string]bool // tagID -> applied
	filtered []*model.Tag
	cursor   int
	visible  bool
	focused  bool
	postID   string
	// private makes Ctrl+N create a private tag rather than a channel tag;
	// Ctrl+T toggles it, and it resets each time the picker opens.
	private bool
	styles  styles.Styles
	width   int
	height  int
}

// New creates a new tag picker model.
func New(s styles.Styles) Model {
	ti := textinput.New()
	s.ThemeTextInput(&ti)
	ti.Placeholder = "Filter tags... (ctrl+n create, ctrl+t private)"
	ti.CharLimit = 50
	m := Model{
		postTags: make(map[string]bool),
		styles:   s,
		input:    ti,
	}
	assert.That(m.postTags != nil, "tagpicker.New: nil applied-tag set")
	return m
}

// Open shows the tag picker for a post.
func (m *Model) Open(postID string, allTags []*model.Tag, appliedTags []*model.Tag) {
	m.visible = true
	m.focused = true
	m.postID = postID
	for _, t := range allTags {
		assert.That(t != nil, "Model.Open: nil tag in the list")
	}
	m.allTags = allTags
	m.postTags = make(map[string]bool)
	for _, t := range appliedTags {
		m.postTags[t.ID] = true
	}
	m.private = false
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.filter()
	assert.That(len(m.postTags) <= len(appliedTags), "Model.Open: more applied tags than given")
	assert.That(len(m.filtered) == len(m.allTags), "Model.Open: empty filter hides tags")
	assert.That(m.cursor == 0, "Model.Open: cursor not on the first tag")
	assert.That(m.input.Focused(), "Model.Open: filter input not focused")
}

// Private reports whether Ctrl+N will create a private tag.
func (m Model) Private() bool { return m.private }

// AddApplied lists a tag just created and applied to postID. The picker
// otherwise goes on listing the tags it opened with, and the new one, made
// from inside it with Ctrl+N, never appears.
func (m *Model) AddApplied(postID string, tag *model.Tag) {
	if !m.visible || postID != m.postID || tag == nil {
		return
	}
	assert.That(m.postTags != nil, "Model.AddApplied: nil applied-tag set")
	if !slices.ContainsFunc(m.allTags, func(t *model.Tag) bool { return t.ID == tag.ID }) {
		// Clip so the append cannot write into the caller's backing array.
		m.allTags = append(slices.Clip(m.allTags), tag)
	}
	m.postTags[tag.ID] = true
	m.filter()
	assert.That(slices.ContainsFunc(m.allTags, func(t *model.Tag) bool { return t.ID == tag.ID }),
		"Model.AddApplied: added tag not listed")
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.AddApplied: cursor outside the list")
}

// Close hides the tag picker.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	assert.That(!m.input.Focused(), "Model.Close: filter input kept focus")
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
	assert.That(m.input.Focused(), "Model.Focus: filter input not focused")
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
	assert.That(!m.input.Focused(), "Model.Blur: filter input kept focus")
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	m.styles = s
	s.ThemeTextInput(&m.input)
}

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w/2 - 8
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.Update: cursor outside the list")

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if next, cmd, handled := m.handleKey(keyMsg); handled {
			return next, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	assert.That(utf8.RuneCountInString(m.input.Value()) <= m.input.CharLimit, "Model.Update: filter over the input limit")
	m.filter()
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.Update: cursor outside the list after filtering")
	return m, cmd
}

// handleKey handles the keys the picker itself uses, and reports whether
// msg was one of them; the rest go to the filter input.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	assert.That(m.visible && m.focused, "Model.handleKey: keys reached a closed or unfocused picker")
	switch msg.Type {
	case tea.KeyEscape:
		m.Close()
		assert.That(!m.visible && !m.focused, "Model.handleKey: Esc left the picker open")
		return m, nil, true
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		assert.That(m.cursor >= 0, "Model.handleKey: cursor above the list")
		return m, nil, true
	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		assert.That(m.cursor <= max(len(m.filtered)-1, 0), "Model.handleKey: cursor below the list")
		return m, nil, true
	case tea.KeyEnter:
		return m, m.toggleCursor(), true
	case tea.KeyCtrlT:
		m.private = !m.private
		return m, nil, true
	case tea.KeyCtrlN:
		return m, m.requestCreate(), true
	}
	return m, nil, false
}

// toggleCursor applies or removes the tag under the cursor.
func (m *Model) toggleCursor() tea.Cmd {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	assert.That(m.cursor >= 0, "Model.toggleCursor: negative cursor")
	assert.That(m.postTags != nil, "Model.toggleCursor: nil applied-tag set")
	tag := m.filtered[m.cursor]
	assert.That(tag != nil, "Model.toggleCursor: nil tag under the cursor")
	applied := m.postTags[tag.ID]
	// Toggle
	if applied {
		m.postTags[tag.ID] = false
	} else {
		m.postTags[tag.ID] = true
	}
	assert.That(m.postTags[tag.ID] != applied, "Model.toggleCursor: tag state did not flip")
	postID := m.postID
	return func() tea.Msg {
		return TagToggledMsg{
			PostID:  postID,
			TagID:   tag.ID,
			TagName: tag.Name,
			Applied: !applied,
		}
	}
}

// requestCreate asks for a new tag named by the input.
func (m *Model) requestCreate() tea.Cmd {
	name := strings.TrimSpace(m.input.Value())
	if name == "" {
		return nil
	}
	assert.That(utf8.RuneCountInString(name) <= m.input.CharLimit, "Model.requestCreate: name over the input limit")
	postID, private := m.postID, m.private
	m.input.Reset()
	m.filter()
	assert.That(m.input.Value() == "", "Model.requestCreate: filter not cleared")
	assert.That(len(m.filtered) == len(m.allTags), "Model.requestCreate: cleared filter hides tags")
	return func() tea.Msg {
		return TagCreateRequestMsg{Name: name, PostID: postID, Private: private}
	}
}

// View renders the tag picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.input.View())
	mode := "new tags: channel (shared with members) · ctrl+t for private"
	if m.private {
		mode = "new tags: private (only you see them) · ctrl+t for channel"
	}
	items = append(items, m.styles.Timestamp.Render("  "+mode), "")

	start, end := listwin.Window(m.cursor, len(m.filtered), listwin.Rows(m.height))
	assert.That(start >= 0 && start <= end && end <= len(m.filtered), "Model.View: window outside the list")
	assert.That(len(m.filtered) == 0 || (m.cursor >= start && m.cursor < end), "Model.View: cursor row not in the window")
	for i := start; i < end; i++ {
		tag := m.filtered[i]
		check := "[ ]"
		if m.postTags[tag.ID] {
			check = "[x]"
		}
		line := check + " #" + tag.Name
		if tag.Private() {
			line += " (private)"
		}
		if i == m.cursor {
			line = m.styles.AutocompleteActive.Render(line)
		} else {
			line = m.styles.AutocompleteItem.Render(line)
		}
		items = append(items, line)
	}
	assert.That(len(items) == 3+end-start, "Model.View: row count does not match the window")

	if len(m.filtered) == 0 && m.input.Value() != "" {
		kind := "channel"
		if m.private {
			kind = "private"
		}
		items = append(items, m.styles.Timestamp.Render("  Press Ctrl+N to create a "+kind+" tag"))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}

func (m *Model) filter() {
	prefix := strings.ToLower(strings.TrimSpace(m.input.Value()))
	if prefix == "" {
		m.filtered = m.allTags
		assert.That(len(m.filtered) == len(m.allTags), "Model.filter: empty filter hides tags")
	} else {
		m.filtered = nil
		for _, t := range m.allTags {
			if strings.HasPrefix(strings.ToLower(t.Name), prefix) {
				m.filtered = append(m.filtered, t)
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(len(m.filtered)-1, 0)
	}
	assert.That(len(m.filtered) <= len(m.allTags), "Model.filter: more matches than tags")
	assert.That(m.cursor >= 0 && m.cursor <= max(len(m.filtered)-1, 0), "Model.filter: cursor outside the list")
}
