// Package activity is a reusable Bubble that shows short-lived activity on
// one line: "chit-agent is responding…".
//
// Set adds or refreshes an item by key, and returns a timer. When the timer
// ends, the parent gives the ExpiredMsg to Expire, and the item goes unless
// a later Set refreshed it. Clear removes an item at once. The set has a
// cap; a Set on a full set drops the oldest item.
//
// The package imports no chit package other than assert and ui/styles.
package activity

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// ExpiredMsg tells that the time of one item ended.
type ExpiredMsg struct {
	key string
	seq uint64
}

type item struct {
	key   string
	label string
	seq   uint64
}

// Model is the set of items.
type Model struct {
	items  []item
	max    int
	ttl    time.Duration
	seq    uint64
	styles styles.Styles
}

// New makes an empty set of at most maxItems items, each of which lasts
// ttl after its last Set.
func New(s styles.Styles, maxItems int, ttl time.Duration) Model {
	assert.That(maxItems > 0, "activity.New: the cap must be positive")
	assert.That(ttl > 0, "activity.New: the time of an item must be positive")
	return Model{max: maxItems, ttl: ttl, styles: s}
}

// Set adds the item key with label, or refreshes it. It returns the timer
// of the item.
func (m *Model) Set(key, label string) tea.Cmd {
	assert.That(key != "" && label != "", "activity.Set: empty key or label")
	m.seq++
	seq := m.seq
	if i := m.index(key); i >= 0 {
		m.items[i].label, m.items[i].seq = label, seq
	} else {
		if len(m.items) >= m.max {
			m.items = m.items[1:]
		}
		m.items = append(m.items, item{key: key, label: label, seq: seq})
	}
	assert.That(len(m.items) <= m.max, "activity.Set: set over its cap")
	return tea.Tick(m.ttl, func(time.Time) tea.Msg { return ExpiredMsg{key: key, seq: seq} })
}

// Clear removes the item key. It reports whether there was one.
func (m *Model) Clear(key string) bool {
	assert.That(key != "", "activity.Clear: empty key")
	i := m.index(key)
	if i < 0 {
		return false
	}
	m.items = append(m.items[:i], m.items[i+1:]...)
	assert.That(m.index(key) < 0, "activity.Clear: a key held twice")
	return true
}

// ClearAll removes each item.
func (m *Model) ClearAll() { m.items = nil }

// Expire removes the item that msg names, unless a later Set refreshed it.
// It reports whether it removed one.
func (m *Model) Expire(msg ExpiredMsg) bool {
	assert.That(msg.seq <= m.seq, "activity.Expire: a timer from the future")
	i := m.index(msg.key)
	if i < 0 || m.items[i].seq != msg.seq {
		return false
	}
	return m.Clear(msg.key)
}

// Len returns the count of items.
func (m Model) Len() int { return len(m.items) }

// Labels returns the labels, oldest first.
func (m Model) Labels() []string {
	out := make([]string, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, it.label)
	}
	assert.That(len(out) == len(m.items), "activity.Labels: a label for each item")
	return out
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "activity.SetStyles: styles not built by styles.New")
	m.styles = s
}

// View draws the labels on one line, cut to width, or "" when the set is
// empty.
func (m Model) View(width int) string {
	assert.That(width >= 0, "activity.View: negative width")
	if len(m.items) == 0 || width == 0 {
		return ""
	}
	line := ansi.Truncate(strings.Join(m.Labels(), " · "), width, "…")
	assert.That(ansi.StringWidth(line) <= width, "activity.View: line wider than the width")
	return m.styles.Timestamp.Render(line)
}

func (m Model) index(key string) int {
	for i, it := range m.items {
		if it.key == key {
			return i
		}
	}
	return -1
}
