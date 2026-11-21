package prompt_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/prompt"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestPrompt_SubmitKeepsItOpen(t *testing.T) {
	m := prompt.New(testutil.Styles(), 50)
	m.SetSize(80, 24)
	if m.Visible() || m.View() != "" {
		t.Fatal("a new prompt is open")
	}
	m.Open("rename:t1", "Rename tag", "old", "Enter to save")
	m, _ = m.Update(runes("er"))
	if m.Value() != "older" || m.Key() != "rename:t1" {
		t.Fatalf("Value = %q, Key = %q", m.Value(), m.Key())
	}
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Rename tag") || !strings.Contains(view, "Enter to save") {
		t.Errorf("View = %q", view)
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg, ok := cmd().(prompt.SubmittedMsg)
	if !ok || msg.Key != "rename:t1" || msg.Value != "older" {
		t.Fatalf("Enter sent %#v", cmd())
	}
	if !m.Visible() {
		t.Error("Enter closed the prompt before the parent checked the value")
	}
}

func TestPrompt_ErrorShowsAndClearsOnEdit(t *testing.T) {
	m := prompt.New(testutil.Styles(), 50)
	m.SetSize(120, 40)
	m.Open("k", "Title", "", "hint")
	m.SetError("that name is in use")
	if v := testutil.StripANSI(m.View()); !strings.Contains(v, "that name is in use") || strings.Contains(v, "hint") {
		t.Errorf("View with an error = %q", v)
	}
	m, _ = m.Update(runes("x"))
	if v := testutil.StripANSI(m.View()); strings.Contains(v, "in use") {
		t.Errorf("an edit kept the error: %q", v)
	}
}

func TestPrompt_EscCancels(t *testing.T) {
	m := prompt.New(testutil.Styles(), 50)
	m.Open("k", "Title", "v", "")
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if msg, ok := cmd().(prompt.CancelledMsg); !ok || msg.Key != "k" {
		t.Errorf("Esc sent %#v", cmd())
	}
	if m.Visible() {
		t.Error("Esc left the prompt open")
	}
	if _, cmd := m.Update(runes("x")); cmd != nil {
		t.Error("a closed prompt took a key")
	}
}

func TestPrompt_LimitTrimAndFocus(t *testing.T) {
	m := prompt.New(testutil.Styles(), 3)
	m.Open("k", "T", "", "")
	m, _ = m.Update(runes("abcdef"))
	if m.Value() != "abc" {
		t.Errorf("Value = %q, want the first 3", m.Value())
	}
	m.Blur()
	m.Focus()
	m.SetStyles(testutil.Styles())
	m.Close()
	m = prompt.New(testutil.Styles(), 20)
	m.Open("k", "T", "  spa  ", "")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(prompt.SubmittedMsg); !ok || msg.Value != "spa" {
		t.Errorf("submitted %#v, want the trimmed value", cmd())
	}
}
