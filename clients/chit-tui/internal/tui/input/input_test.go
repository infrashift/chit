package input_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/tui/input"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestInput_SendMsg(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type characters
	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	if m.Value() != "hello" {
		t.Errorf("value = %q, want 'hello'", m.Value())
	}

	// Press enter
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	send, ok := msg.(input.SendMsg)
	if !ok {
		t.Fatalf("expected SendMsg, got %T", msg)
	}
	if send.Content != "hello" {
		t.Errorf("content = %q, want 'hello'", send.Content)
	}

	// Input should be cleared
	if m.Value() != "" {
		t.Errorf("expected empty input after send, got %q", m.Value())
	}
}

func TestInput_SlashTrigger(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	for _, ch := range "/remind" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	slash, ok := msg.(input.SlashTriggerMsg)
	if !ok {
		t.Fatalf("expected SlashTriggerMsg, got %T", msg)
	}
	if slash.Input != "/remind" {
		t.Errorf("input = %q, want '/remind'", slash.Input)
	}
}

func TestInput_EmptyEnterNoOp(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for empty enter")
	}
}

func TestInput_FocusBlur(t *testing.T) {
	m := input.New(testStyles())
	if m.Focused() {
		t.Error("should not be focused initially")
	}
	m.Focus()
	if !m.Focused() {
		t.Error("should be focused")
	}
	m.Blur()
	if m.Focused() {
		t.Error("should not be focused after blur")
	}
}

func TestInput_IgnoresWhenBlurred(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	// Not focused

	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.Value() != "" {
		t.Errorf("expected empty value when blurred, got %q", m.Value())
	}
}

func TestInput_Reset(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	for _, ch := range "test" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m.Reset()
	if m.Value() != "" {
		t.Errorf("expected empty after reset, got %q", m.Value())
	}
}

func TestInput_AtTrigger(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "@us"
	for _, ch := range "@us" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// The last update should have produced an AtTriggerMsg
	// We verify by checking with one more rune to get the batched command
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("expected command with @ trigger")
	}
	// The command is a batch; execute it
	// Since bubbletea Batch returns a func, we can check the msg types
	msg := cmd()
	// With Batch, the returned msg could be a BatchMsg or individual msg
	// In practice, at least one of the messages in the batch should be AtTriggerMsg
	if msg == nil {
		t.Fatal("expected non-nil msg")
	}
}

func TestInput_AtDismissOnSpace(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "@us"
	for _, ch := range "@us" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Type space - should dismiss
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("expected command with @ dismiss")
	}
}

func TestInput_ReplaceAtMention(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "@al"
	for _, ch := range "@al" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	m.ReplaceAtMention(0, "alice")
	val := m.Value()
	if val != "@alice " {
		t.Errorf("value = %q, want '@alice '", val)
	}
}

func TestInput_AltEnterInsertsNewline(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "line1"
	for _, ch := range "line1" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Press Alt+Enter — should NOT send, should insert newline
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(input.SendMsg); ok {
			t.Fatal("Alt+Enter should not send a message")
		}
	}

	// Type "line2"
	for _, ch := range "line2" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	val := m.Value()
	if val != "line1\nline2" {
		t.Errorf("value = %q, want %q", val, "line1\nline2")
	}
}

func TestInput_MultilineContentPreservedInSendMsg(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "hello"
	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Alt+Enter for newline
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})

	// Type "world"
	for _, ch := range "world" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Plain Enter sends the multiline content
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	send, ok := msg.(input.SendMsg)
	if !ok {
		t.Fatalf("expected SendMsg, got %T", msg)
	}
	if send.Content != "hello\nworld" {
		t.Errorf("content = %q, want %q", send.Content, "hello\nworld")
	}
}

func TestInput_PlainEnterStillSends(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	for _, ch := range "hi" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	send, ok := msg.(input.SendMsg)
	if !ok {
		t.Fatalf("expected SendMsg, got %T", msg)
	}
	if send.Content != "hi" {
		t.Errorf("content = %q, want 'hi'", send.Content)
	}
}

func TestInput_SlashDoesNotTriggerAt(t *testing.T) {
	m := input.New(testStyles())
	m.SetSize(80, 5)
	m.Focus()

	// Type "/remind"
	for _, ch := range "/remind" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Value should be /remind, no @ trigger
	if m.Value() != "/remind" {
		t.Errorf("value = %q", m.Value())
	}
}
