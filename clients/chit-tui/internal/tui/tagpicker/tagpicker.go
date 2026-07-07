package tagpicker

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// TagToggledMsg is sent when a tag is toggled on/off for a post.
type TagToggledMsg struct {
	PostID  string
	TagID   string
	TagName string
	Applied bool
}

// TagCreateRequestMsg is sent when the user wants to create a new tag.
type TagCreateRequestMsg struct {
	Name   string
	PostID string
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
	styles   styles.Styles
	width    int
	height   int
}

// New creates a new tag picker model.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter tags... (ctrl+n to create)"
	ti.CharLimit = 50
	return Model{
		postTags: make(map[string]bool),
		styles:   s,
		input:    ti,
	}
}

// Open shows the tag picker for a post.
func (m *Model) Open(postID string, allTags []*model.Tag, appliedTags []*model.Tag) {
	m.visible = true
	m.focused = true
	m.postID = postID
	m.allTags = allTags
	m.postTags = make(map[string]bool)
	for _, t := range appliedTags {
		m.postTags[t.ID] = true
	}
	m.input.Reset()
	m.input.Focus()
	m.cursor = 0
	m.filter()
}

// Close hides the tag picker.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

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

	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.Type {
		case tea.KeyEscape:
			m.Close()
			return m, nil
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				tag := m.filtered[m.cursor]
				applied := m.postTags[tag.ID]
				// Toggle
				if applied {
					m.postTags[tag.ID] = false
				} else {
					m.postTags[tag.ID] = true
				}
				postID := m.postID
				return m, func() tea.Msg {
					return TagToggledMsg{
						PostID:  postID,
						TagID:   tag.ID,
						TagName: tag.Name,
						Applied: !applied,
					}
				}
			}
			return m, nil
		case tea.KeyCtrlN:
			name := strings.TrimSpace(m.input.Value())
			if name != "" {
				postID := m.postID
				m.input.Reset()
				return m, func() tea.Msg {
					return TagCreateRequestMsg{Name: name, PostID: postID}
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filter()
	return m, cmd
}

// View renders the tag picker overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	var items []string
	items = append(items, m.input.View())
	items = append(items, "")

	maxItems := min(len(m.filtered), max((m.height/2)-4, 5))
	for i := range maxItems {
		tag := m.filtered[i]
		check := "[ ]"
		if m.postTags[tag.ID] {
			check = "[x]"
		}
		line := check + " #" + tag.Name
		if i == m.cursor {
			line = m.styles.AutocompleteActive.Render(line)
		} else {
			line = m.styles.AutocompleteItem.Render(line)
		}
		items = append(items, line)
	}

	if len(m.filtered) == 0 && m.input.Value() != "" {
		items = append(items, m.styles.Timestamp.Render("  Press Ctrl+N to create tag"))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(body)
}

func (m *Model) filter() {
	prefix := strings.ToLower(strings.TrimSpace(m.input.Value()))
	if prefix == "" {
		m.filtered = m.allTags
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
}
