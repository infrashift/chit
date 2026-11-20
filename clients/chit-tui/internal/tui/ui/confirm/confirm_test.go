package confirm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/confirm"
)

func keyOf(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func yesD() key.Binding { return key.NewBinding(key.WithKeys("d")) }

func TestConfirm_NothingAsked(t *testing.T) {
	m := confirm.New(testutil.Styles(), time.Second)
	if subject, a := m.Answer(keyOf('d')); a != confirm.NotAsked || subject != "" {
		t.Errorf("Answer = %q, %v; want NotAsked", subject, a)
	}
	if m.Asking() || m.View() != "" {
		t.Error("a new Model asks something")
	}
}

func TestConfirm_YesKey(t *testing.T) {
	m := confirm.New(testutil.Styles(), time.Second)
	if cmd := m.Ask("delete:p1", "delete it?", yesD()); cmd == nil {
		t.Fatal("Ask returned no timer")
	}
	if !m.Asking() || m.Subject() != "delete:p1" || m.Question() != "delete it?" {
		t.Fatalf("after Ask: %q %q %v", m.Subject(), m.Question(), m.Asking())
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "delete it?") {
		t.Errorf("View = %q", m.View())
	}
	subject, a := m.Answer(keyOf('d'))
	if a != confirm.Yes || subject != "delete:p1" {
		t.Errorf("Answer = %q, %v; want delete:p1, Yes", subject, a)
	}
	if m.Asking() {
		t.Error("the question stayed open after Yes")
	}
}

func TestConfirm_OtherKeyIsNo(t *testing.T) {
	m := confirm.New(testutil.Styles(), time.Second)
	m.Ask("s", "q?", yesD())
	if subject, a := m.Answer(keyOf('k')); a != confirm.No || subject != "s" {
		t.Errorf("Answer = %q, %v; want s, No", subject, a)
	}
	if _, a := m.Answer(keyOf('d')); a != confirm.NotAsked {
		t.Error("a second key answered a closed question")
	}
}

func TestConfirm_Expire(t *testing.T) {
	m := confirm.New(testutil.Styles(), time.Millisecond)
	first := m.Ask("a", "first?", yesD())
	firstMsg, ok := first().(confirm.ExpiredMsg)
	if !ok {
		t.Fatal("the timer sent no ExpiredMsg")
	}
	m.Ask("b", "second?", yesD())
	if m.Expire(firstMsg) {
		t.Error("the timer of a replaced question closed the new one")
	}
	if m.Subject() != "b" {
		t.Fatalf("Subject = %q, want b", m.Subject())
	}
	second := m.Ask("b", "again?", yesD())
	msg, ok := second().(confirm.ExpiredMsg)
	if !ok || !m.Expire(msg) || m.Asking() {
		t.Error("the timer of the open question did not close it")
	}
	if m.Expire(msg) {
		t.Error("Expire closed a question twice")
	}
}

func TestConfirm_CancelAndStyles(t *testing.T) {
	m := confirm.New(testutil.Styles(), time.Second)
	m.Ask("s", "q?", yesD())
	m.SetStyles(testutil.Styles())
	m.Cancel()
	if m.Asking() || m.View() != "" {
		t.Error("Cancel left the question open")
	}
}
