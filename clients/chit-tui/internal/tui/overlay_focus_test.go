package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
)

// Regression tests for the focus-orphan bug: Esc-closing an overlay must
// restore focus to a live component, not leave it on the closed overlay.

// enterReachesViewport reports whether Enter is delivered to the focused
// viewport (it emits a thread-fetch command for the selected post).
func enterReachesViewport(t *testing.T, m tui.Model) bool {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return cmd != nil
}

// typingReachesInput reports whether a typed rune lands in the input box.
func typingReachesInput(t *testing.T, m tui.Model) bool {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zq")})
	view := testutil.StripANSI(updated.(tui.Model).View())
	return strings.Contains(view, "zq")
}

func TestModel_EscClosingTagPickerRestoresViewportFocus(t *testing.T) {
	m := setupModel(t)

	// Focus the viewport and open the tag picker on the selected post.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	if !enterReachesViewport(t, m) {
		t.Error("expected viewport to be focused after Esc-closing the tag picker")
	}
}

func TestModel_EscClosingChCreatorRestoresInputFocus(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	if !typingReachesInput(t, m) {
		t.Error("expected input to be focused after Esc-closing the channel creator")
	}
}

func TestModel_EscClosingSkinPickerRestoresInputFocus(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/skin"})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	if !typingReachesInput(t, m) {
		t.Error("expected input to be focused after Esc-closing the skin picker")
	}
}

func TestModel_EscClosingMemberPickerCreatesPendingChannel(t *testing.T) {
	m := setupModel(t)

	// Submitting a private channel opens the member picker with the channel
	// pending.
	pending := &model.Channel{DisplayName: "Secret", Name: "secret", Type: model.ChannelPrivate, TeamID: "t1"}
	updated, _ := m.Update(chcreator.ChannelSubmittedMsg{Channel: pending})
	m = updated.(tui.Model)

	// Esc skips member selection but still creates the channel: the picker
	// emits a cancel message, whose handler issues CreateChannel.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected commands from Esc on the member picker")
	}
	created := false
	for _, msg := range runCmds(cmd) {
		updated, next := m.Update(msg)
		m = updated.(tui.Model)
		for _, result := range runCmds(next) {
			if _, ok := result.(tui.ChannelCreatedMsg); ok {
				created = true
			}
		}
	}
	if !created {
		t.Error("expected Esc on the member picker to create the pending private channel")
	}

	if !typingReachesInput(t, m) {
		t.Error("expected input to be focused after Esc-closing the member picker")
	}
}

func TestModel_EscClosingMemberPickerClearsPendingState(t *testing.T) {
	m := setupModel(t)

	pending := &model.Channel{DisplayName: "Secret", Name: "secret", Type: model.ChannelPrivate, TeamID: "t1"}
	updated, _ := m.Update(chcreator.ChannelSubmittedMsg{Channel: pending})
	m = updated.(tui.Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)
	// Process the cancel message so the pending channel is consumed.
	for _, msg := range runCmds(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(tui.Model)
	}

	// A repeated cancel (nothing pending) must not create anything.
	_, cmd = m.Update(dmpicker.CancelledMsg{})
	for _, msg := range runCmds(cmd) {
		if _, ok := msg.(tui.ChannelCreatedMsg); ok {
			t.Error("pending channel state leaked: repeated cancel created a channel")
		}
	}
}

func TestModel_HelpCloseKeepsPriorFocus(t *testing.T) {
	m := setupModel(t)

	// Focus the viewport, open help with "?", close it with a key.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(tui.Model)

	if !enterReachesViewport(t, m) {
		t.Error("expected viewport focus to survive opening and closing help")
	}
}
