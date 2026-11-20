package chcreator_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
)

// ISSUE-013: the active field blinks, and a blink is no edit, so the
// validation error stays on screen.
func TestChCreator_BlinkKeepsTheValidationError(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")
	m, typed := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	blink, ok := creatorBlinkOf(typed)
	if !ok {
		t.Fatal("typing in the display name gave no cursor blink")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m.View(), "Invalid name") {
		t.Fatalf("setup: no validation error:\n%s", m.View())
	}

	m, next := m.Update(blink)
	if !strings.Contains(m.View(), "Invalid name") {
		t.Errorf("a blink cleared the validation error:\n%s", m.View())
	}
	if _, ok := creatorBlinkOf(next); !ok {
		t.Error("the active field did not schedule its next blink")
	}
}

// creatorBlinkOf runs cmd and returns its cursor blink. A blink is a tick
// of about half a second, so the command has two seconds.
func creatorBlinkOf(cmd tea.Cmd) (cursor.BlinkMsg, bool) {
	if cmd == nil {
		return cursor.BlinkMsg{}, false
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		blink, ok := msg.(cursor.BlinkMsg)
		return blink, ok
	case <-time.After(2 * time.Second):
		return cursor.BlinkMsg{}, false
	}
}
