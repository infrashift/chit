// Package confirm is a reusable Bubble that asks one y/N question.
//
// The parent calls Ask with a subject and the key that answers yes. While
// the question is open, the parent gives each key press to Answer first.
// The yes key answers Yes. Each other key answers No: the question closes,
// and the key goes on to do its usual work. The question also closes when
// its time ends. The parent receives ExpiredMsg and gives it to Expire.
//
// Answer returns the result at once, so the parent acts in the same Update.
// The package imports no chit package other than assert and ui/styles.
package confirm

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Answer is what a key press tells an open question.
type Answer int

const (
	// NotAsked tells that no question is open. The key is the parent's.
	NotAsked Answer = iota
	// Yes tells that the key was the yes key.
	Yes
	// No tells that the key was another key. The question is closed.
	No
)

// ExpiredMsg tells that the time of one question ended.
type ExpiredMsg struct {
	seq uint64
}

// Model is one y/N question at a time.
type Model struct {
	subject  string
	question string
	yes      key.Binding
	seq      uint64
	ttl      time.Duration
	styles   styles.Styles
}

// New makes a Model whose questions stay open for ttl.
func New(s styles.Styles, ttl time.Duration) Model {
	assert.That(ttl > 0, "confirm.New: the time of a question must be positive")
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "confirm.New: styles not built by styles.New")
	return Model{ttl: ttl, styles: s}
}

// Ask opens a question about subject and replaces an open question. It
// returns the command that ends the question after its time.
func (m *Model) Ask(subject, question string, yes key.Binding) tea.Cmd {
	assert.That(subject != "" && question != "", "confirm.Ask: empty subject or question")
	assert.That(len(yes.Keys()) > 0, "confirm.Ask: the yes binding has no key")
	m.seq++
	m.subject, m.question, m.yes = subject, question, yes
	seq := m.seq
	return tea.Tick(m.ttl, func(time.Time) tea.Msg { return ExpiredMsg{seq: seq} })
}

// Asking reports whether a question is open.
func (m Model) Asking() bool { return m.subject != "" }

// Subject returns the subject of the open question, or "".
func (m Model) Subject() string { return m.subject }

// Question returns the text of the open question, or "".
func (m Model) Question() string { return m.question }

// Answer gives a key press to the open question. With Yes or No, the
// question closes, and Answer returns its subject.
func (m *Model) Answer(msg tea.KeyMsg) (string, Answer) {
	if !m.Asking() {
		return "", NotAsked
	}
	subject := m.subject
	m.Cancel()
	assert.That(!m.Asking() && subject != "", "confirm.Answer: the question stayed open")
	if key.Matches(msg, m.yes) {
		return subject, Yes
	}
	return subject, No
}

// Cancel closes the open question, if there is one.
func (m *Model) Cancel() {
	m.subject, m.question = "", ""
	assert.That(!m.Asking(), "confirm.Cancel: the question stayed open")
}

// Expire closes the question that msg names, if it is still open. It
// reports whether it closed one.
func (m *Model) Expire(msg ExpiredMsg) bool {
	assert.That(msg.seq <= m.seq, "confirm.Expire: a question from the future")
	if !m.Asking() || msg.seq != m.seq {
		return false
	}
	m.Cancel()
	return true
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) {
	assert.That(s.MarkdownStyleConfig.CodeBlock.Chroma != nil, "confirm.SetStyles: styles not built by styles.New")
	m.styles = s
}

// View returns the open question in the error style, or "".
func (m Model) View() string {
	if !m.Asking() {
		return ""
	}
	out := m.styles.ErrorText.Render(m.question)
	assert.That(out != "", "confirm.View: an open question rendered empty")
	return out
}
