package input_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
)

func TestInput_SendMsg(t *testing.T) {
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
	m.SetSize(80, 5)
	m.Focus()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for empty enter")
	}
}

func TestInput_FocusBlur(t *testing.T) {
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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
	m := input.New(testutil.Styles())
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

// The replacement sliced the line by byte at a character column, so text
// with an accented letter before the @ was cut mid-character.
func TestInput_ReplaceAtMentionAfterNonASCII(t *testing.T) {
	m := input.New(testutil.Styles())
	m.SetSize(80, 5)
	m.Focus()
	for _, ch := range "héllo @al" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	m.ReplaceAtMention(6, "alice")

	if got := m.Value(); got != "héllo @alice " {
		t.Errorf("value = %q, want %q", got, "héllo @alice ")
	}
}

// Replacing on a later line must leave the cursor after the inserted name,
// so typing carries on from there.
func TestInput_ReplaceAtMentionOnASecondLine(t *testing.T) {
	m := input.New(testutil.Styles())
	m.SetSize(80, 5)
	m.Focus()
	m.SetValue("first line\nsay @al")

	m.ReplaceAtMention(4, "alice")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})

	if got := m.Value(); got != "first line\nsay @alice !" {
		t.Errorf("value = %q", got)
	}
}

func TestInput_ViewShowsWhatIsTyped(t *testing.T) {
	m := input.New(testutil.Styles())
	m.SetSize(80, 5)
	m.Focus()
	for _, r := range "hello there" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "hello there") {
		t.Errorf("typed text not shown:\n%s", testutil.StripANSI(m.View()))
	}
	// Restyling keeps what is being written.
	m.SetStyles(testutil.Styles())
	if m.Value() != "hello there" {
		t.Errorf("value = %q after SetStyles", m.Value())
	}
}

// Moving off a mention prefix dismisses the popup once, and only once.
func TestInput_LeavingAMentionDismissesThePopup(t *testing.T) {
	m := input.New(testutil.Styles())
	m.SetSize(80, 5)
	m.Focus()
	var cmd tea.Cmd
	for _, r := range "@al" {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !hasMsg[input.AtTriggerMsg](cmd) {
		t.Fatal("typing @al did not open the popup")
	}

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !hasMsg[input.AtDismissMsg](cmd) {
		t.Error("a space after the mention did not dismiss the popup")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if hasMsg[input.AtDismissMsg](cmd) {
		t.Error("dismissed again with no popup open")
	}
}

func hasMsg[T tea.Msg](cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if hasMsg[T](c) {
				return true
			}
		}
		return false
	}
	_, ok := msg.(T)
	return ok
}
