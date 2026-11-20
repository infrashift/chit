package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/activity"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/confirm"
)

// The y/N questions of the root model (ui/confirm). A subject is a prefix
// and the ID of the thing that the question is about.
const (
	// questionTTL is how long a question stays open, as long as an error.
	questionTTL = 10 * time.Second

	subjectDeletePost  = "delete-post:"
	questionDeletePost = "delete this message? Press d again to confirm, any other key to cancel"
)

// routeBubbleMsg routes the messages of the reusable Bubbles in ui/.
func (m Model) routeBubbleMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeBubbleMsg: not signed in")
	assert.That(m.scope != nil, "routeBubbleMsg: nil request scope")
	switch msg := msg.(type) {
	case confirm.ExpiredMsg:
		m.confirm.Expire(msg)
		return m, nil, true
	case activity.ExpiredMsg:
		m.activity.Expire(msg)
		return m, nil, true
	}
	return m, nil, false
}

// answerQuestion gives a key press to the open question. Only a yes that
// still applies reports the key as handled. Each other key closes the
// question and then does its usual work.
func (m *Model) answerQuestion(msg tea.KeyMsg) (tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "answerQuestion: not signed in")
	subject, answer := m.confirm.Answer(msg)
	if answer != confirm.Yes {
		return nil, false
	}
	assert.That(subject != "", "answerQuestion: a yes without a subject")
	if id, ok := strings.CutPrefix(subject, subjectDeletePost); ok {
		return m.deleteConfirmedPost(id)
	}
	return nil, false
}

// deleteConfirmedPost deletes post id, if it is still the user's own post
// under the cursor of the history pane.
func (m *Model) deleteConfirmedPost(id string) (tea.Cmd, bool) {
	assert.That(id != "", "deleteConfirmedPost: empty post ID")
	if m.focus != FocusViewport {
		return nil, false
	}
	p := m.ownSelectedPost()
	if p == nil || p.ID != id {
		return nil, false
	}
	assert.That(m.client != nil && m.scope != nil, "deleteConfirmedPost: no client or request scope")
	return DeletePost(m.reqCtx(), m.client, p.ID), true
}
