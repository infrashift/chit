package tui_test

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
)

// ISSUE-013: after a mention pick, the root model returns the blink that
// the backspaces started, and routes that blink back to the input, which
// answers with the next blink. Before, the cursor stopped until the next key.
func TestModel_MentionPickKeepsTheCursorBlinking(t *testing.T) {
	m := setupModel(t)
	for _, r := range "hi @al" {
		m, _ = step(t, m, key(r))
	}

	m, cmd := step(t, m, mention.UserSelectedMsg{Username: "alice", StartCol: 3})
	blink, ok := rootBlinkOf(cmd)
	if !ok {
		t.Fatal("the mention pick returned no cursor blink")
	}
	_, next := step(t, m, blink)
	if _, ok := rootBlinkOf(next); !ok {
		t.Fatal("the input did not answer the blink, so it was stale or not routed")
	}
}

// ISSUE-013: the root model gives each cursor blink to the input, so the
// cursor blinks after a key press instead of staying a steady block.
func TestModel_CursorBlinksAfterTyping(t *testing.T) {
	m := setupModel(t)
	m, cmd := step(t, m, key('h'))
	for i := range 3 {
		blink, ok := rootBlinkOf(cmd)
		if !ok {
			t.Fatalf("blink %d: no cursor blink to deliver", i)
		}
		m, cmd = step(t, m, blink)
	}
	if _, ok := rootBlinkOf(cmd); !ok {
		t.Fatal("the blink cycle stopped after three blinks")
	}
}

// The first user of a session starts the blink: NewModel could not.
func TestModel_SignInStartsTheCursorBlink(t *testing.T) {
	m, _ := step(t, testModel(), tea.WindowSizeMsg{Width: 120, Height: 40})
	_, cmd := step(t, m, tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	if _, ok := rootBlinkOf(cmd); !ok {
		t.Fatal("loading the first user did not start the cursor blink")
	}
}

// A blink is no edit: it must not send the mention trigger again while the
// popup is open, or each blink starts a new user search.
func TestModel_BlinkDoesNotRetriggerTheMentionPopup(t *testing.T) {
	m := setupModel(t)
	var cmd tea.Cmd
	for _, r := range "hi @al" {
		m, cmd = step(t, m, key(r))
	}
	blink, ok := rootBlinkOf(cmd)
	if !ok {
		t.Fatal("typing gave no cursor blink")
	}
	_, next := step(t, m, blink)
	for _, msg := range messagesOf(next) {
		if _, ok := msg.(input.AtTriggerMsg); ok {
			t.Fatal("a blink sent the mention trigger again")
		}
	}
}

// rootBlinkOf runs cmd, and the commands of a batch, and returns the first
// cursor blink among their messages. A blink is a tick of about half a
// second, so each command has two seconds.
func rootBlinkOf(cmd tea.Cmd) (cursor.BlinkMsg, bool) {
	if cmd == nil {
		return cursor.BlinkMsg{}, false
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(2 * time.Second):
		return cursor.BlinkMsg{}, false
	}
	if blink, ok := msg.(cursor.BlinkMsg); ok {
		return blink, true
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return cursor.BlinkMsg{}, false
	}
	for _, sub := range batch {
		if blink, ok := rootBlinkOf(sub); ok {
			return blink, true
		}
	}
	return cursor.BlinkMsg{}, false
}
