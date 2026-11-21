// Package chips is a reusable Bubble that holds a short, ordered list of
// chips: the people of a group, the files of a post, the tags of a channel.
//
// A chip has an ID and a label. The list has a cap, holds each ID once, and
// keeps the order of the adds. Backspace on a focused list removes the last
// chip and sends RemovedMsg. View wraps the chips to the width.
//
// The package imports no chit package other than assert and ui/styles.
package chips

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Role chooses the style of the chips.
type Role int

const (
	// RoleMention draws a chip as a mention, for people.
	RoleMention Role = iota
	// RoleTag draws a chip as a tag badge.
	RoleTag
	// RolePlain draws a chip in the muted text style, for files.
	RolePlain
)

// Chip is one item of the list.
type Chip struct {
	ID    string
	Label string
}

// RemovedMsg tells that Backspace removed a chip.
type RemovedMsg struct {
	Chip Chip
}

// Model is the list of chips.
type Model struct {
	chips   []Chip
	max     int
	role    Role
	focused bool
	width   int
	styles  styles.Styles
}

// New makes an empty list of at most maxChips chips.
func New(s styles.Styles, role Role, maxChips int) Model {
	assert.That(maxChips > 0, "chips.New: the cap must be positive")
	assert.That(role >= RoleMention && role <= RolePlain, "chips.New: unknown role")
	return Model{max: maxChips, role: role, styles: s}
}

// Add appends c. It reports false, and changes nothing, when the list is
// full or already holds c.ID.
func (m *Model) Add(c Chip) bool {
	assert.That(c.ID != "", "chips.Add: empty ID")
	if m.Has(c.ID) || len(m.chips) >= m.max {
		return false
	}
	m.chips = append(m.chips, c)
	assert.That(len(m.chips) <= m.max, "chips.Add: list over its cap")
	return true
}

// Toggle removes c when the list holds it, and adds it when not. It reports
// whether the list holds c after the call.
func (m *Model) Toggle(c Chip) bool {
	assert.That(len(m.chips) <= m.max, "chips.Toggle: list over its cap")
	assert.That(c.ID != "", "chips.Toggle: empty ID")
	if m.Remove(c.ID) {
		return false
	}
	return m.Add(c)
}

// Remove removes the chip with id. It reports whether there was one.
func (m *Model) Remove(id string) bool {
	assert.That(id != "", "chips.Remove: empty ID")
	for i, c := range m.chips {
		if c.ID == id {
			m.chips = append(m.chips[:i], m.chips[i+1:]...)
			assert.That(!m.Has(id), "chips.Remove: an ID held twice")
			return true
		}
	}
	return false
}

// Pop removes the last chip and returns it, and false when the list is
// empty.
func (m *Model) Pop() (Chip, bool) {
	assert.That(len(m.chips) <= m.max, "chips.Pop: list over its cap")
	if len(m.chips) == 0 {
		return Chip{}, false
	}
	last := m.chips[len(m.chips)-1]
	m.chips = m.chips[:len(m.chips)-1]
	assert.That(!m.Has(last.ID), "chips.Pop: an ID held twice")
	return last, true
}

// Has reports whether the list holds id.
func (m Model) Has(id string) bool {
	assert.That(id != "", "chips.Has: empty ID")
	assert.That(len(m.chips) <= m.max, "chips.Has: list over its cap")
	for _, c := range m.chips {
		if c.ID == id {
			return true
		}
	}
	return false
}

// Set replaces the list with cs, to the cap, without duplicate IDs.
func (m *Model) Set(cs []Chip) {
	assert.That(m.max > 0, "chips.Set: list not made by New")
	m.chips = nil
	for _, c := range cs {
		m.Add(c)
	}
	assert.That(len(m.chips) <= m.max, "chips.Set: list over its cap")
}

// Clear removes each chip.
func (m *Model) Clear() { m.chips = nil }

// Chips returns a copy of the list.
func (m Model) Chips() []Chip {
	out := make([]Chip, len(m.chips))
	n := copy(out, m.chips)
	assert.That(n == len(m.chips), "chips.Chips: list not copied whole")
	return out
}

// IDs returns the IDs of the list, in order.
func (m Model) IDs() []string {
	out := make([]string, 0, len(m.chips))
	for _, c := range m.chips {
		out = append(out, c.ID)
	}
	assert.That(len(out) == len(m.chips), "chips.IDs: an ID for each chip")
	return out
}

// Len returns the count of chips.
func (m Model) Len() int { return len(m.chips) }

// Full reports whether the list is at its cap.
func (m Model) Full() bool { return len(m.chips) >= m.max }

// Focus gives the list the keys.
func (m *Model) Focus() { m.focused = true }

// Blur takes the keys from the list.
func (m *Model) Blur() { m.focused = false }

// Focused reports whether the list has the keys.
func (m Model) Focused() bool { return m.focused }

// SetWidth sets the width that View wraps to. Zero means no wrap.
func (m *Model) SetWidth(w int) {
	assert.That(w >= 0, "chips.SetWidth: negative width")
	m.width = w
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "chips.SetStyles: styles not built by styles.New")
	m.styles = s
}

// Update removes the last chip on Backspace when the list has focus.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok || !m.focused || k.Type != tea.KeyBackspace {
		return m, nil
	}
	c, ok := m.Pop()
	if !ok {
		return m, nil
	}
	assert.That(c.ID != "", "chips.Update: removed a chip without an ID")
	return m, func() tea.Msg { return RemovedMsg{Chip: c} }
}

// style returns the style of the role.
func (m Model) style() lipgloss.Style {
	switch m.role {
	case RoleTag:
		return m.styles.TagBadge
	case RolePlain:
		return m.styles.Timestamp
	}
	assert.That(m.role == RoleMention, "chips.style: unknown role")
	return m.styles.MentionText
}

// View draws the chips on as many lines as the width needs, or "" when the
// list is empty.
func (m Model) View() string {
	if len(m.chips) == 0 {
		return ""
	}
	st := m.style()
	var lines []string
	line, lineW := "", 0
	for _, c := range m.chips {
		r := st.Render(c.Label)
		w := lipgloss.Width(r)
		if lineW > 0 && m.width > 0 && lineW+1+w > m.width {
			lines, line, lineW = append(lines, line), "", 0
		}
		if lineW > 0 {
			line, lineW = line+" ", lineW+1
		}
		line, lineW = line+r, lineW+w
	}
	lines = append(lines, line)
	assert.That(len(lines) <= len(m.chips), "chips.View: more lines than chips")
	return strings.Join(lines, "\n")
}
