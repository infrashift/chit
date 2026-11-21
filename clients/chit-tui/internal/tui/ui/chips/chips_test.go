package chips_test

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/chips"
)

func chip(id string) chips.Chip { return chips.Chip{ID: id, Label: "@" + id} }

func TestChips_AddCapAndDuplicates(t *testing.T) {
	m := chips.New(testutil.Styles(), chips.RoleMention, 2)
	if !m.Add(chip("a")) || !m.Add(chip("b")) {
		t.Fatal("Add refused room in the list")
	}
	if m.Add(chip("c")) {
		t.Error("Add went past the cap")
	}
	if !m.Full() || m.Len() != 2 {
		t.Errorf("Len = %d, Full = %v", m.Len(), m.Full())
	}
	m.Remove("b")
	if m.Add(chip("a")) {
		t.Error("Add took an ID twice")
	}
	if !slices.Equal(m.IDs(), []string{"a"}) {
		t.Errorf("IDs = %v", m.IDs())
	}
}

func TestChips_ToggleRemovePopSetClear(t *testing.T) {
	m := chips.New(testutil.Styles(), chips.RoleTag, 3)
	if !m.Toggle(chip("a")) || m.Toggle(chip("a")) || m.Has("a") {
		t.Error("Toggle did not add then remove")
	}
	if m.Remove("missing") {
		t.Error("Remove found a missing chip")
	}
	m.Set([]chips.Chip{chip("x"), chip("y"), chip("x"), chip("z"), chip("w")})
	if !slices.Equal(m.IDs(), []string{"x", "y", "z"}) {
		t.Errorf("Set = %v, want [x y z]", m.IDs())
	}
	got := m.Chips()
	got[0].Label = "changed"
	if m.Chips()[0].Label == "changed" {
		t.Error("Chips gave out the list itself")
	}
	if c, ok := m.Pop(); !ok || c.ID != "z" {
		t.Errorf("Pop = %v, %v", c, ok)
	}
	m.Clear()
	if _, ok := m.Pop(); ok || m.Len() != 0 {
		t.Error("Clear left chips")
	}
}

func TestChips_BackspaceWhenFocused(t *testing.T) {
	m := chips.New(testutil.Styles(), chips.RolePlain, 4)
	m.Add(chip("a"))
	m.Add(chip("b"))
	bs := tea.KeyMsg{Type: tea.KeyBackspace}
	m, cmd := m.Update(bs)
	if cmd != nil || m.Len() != 2 {
		t.Fatal("Backspace removed a chip from a list without focus")
	}
	m.Focus()
	if !m.Focused() {
		t.Fatal("Focus did not focus")
	}
	m, cmd = m.Update(bs)
	if cmd == nil {
		t.Fatal("Backspace sent nothing")
	}
	if msg, ok := cmd().(chips.RemovedMsg); !ok || msg.Chip.ID != "b" {
		t.Errorf("msg = %#v, want RemovedMsg for b", cmd())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.Len() != 1 {
		t.Error("another key removed a chip")
	}
	m.Pop()
	if _, cmd = m.Update(bs); cmd != nil {
		t.Error("Backspace on an empty list sent a message")
	}
	m.Blur()
	if m.Focused() {
		t.Error("Blur did not blur")
	}
}

func TestChips_ViewWraps(t *testing.T) {
	m := chips.New(testutil.Styles(), chips.RoleMention, 8)
	if m.View() != "" {
		t.Error("an empty list drew something")
	}
	for _, id := range []string{"alice", "bob", "carol", "dave"} {
		m.Add(chip(id))
	}
	if got := testutil.StripANSI(m.View()); got != "@alice @bob @carol @dave" {
		t.Errorf("one line = %q", got)
	}
	m.SetWidth(12)
	m.SetStyles(testutil.Styles())
	lines := strings.Split(testutil.StripANSI(m.View()), "\n")
	if len(lines) < 2 {
		t.Fatalf("no wrap at width 12: %q", lines)
	}
	for _, l := range lines {
		if len(l) > 12 {
			t.Errorf("line %q is wider than 12", l)
		}
	}
}
