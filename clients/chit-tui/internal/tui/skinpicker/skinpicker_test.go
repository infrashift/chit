package skinpicker_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestSkinPicker_OpenClose(t *testing.T) {
	m := skinpicker.New(testStyles())
	if m.Visible() {
		t.Error("should not be visible initially")
	}
	m.Open()
	if !m.Visible() {
		t.Error("should be visible after Open")
	}
	m.Close()
	if m.Visible() {
		t.Error("should not be visible after Close")
	}
}

func TestSkinPicker_EscapeCloses(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("escape should close picker")
	}
}

func TestSkinPicker_NavigateAndSelect(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin", "kanagawa"})
	m.SetSize(80, 40)
	m.Open()

	// Navigate down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	selected, ok := msg.(skinpicker.SkinSelectedMsg)
	if !ok {
		t.Fatalf("expected SkinSelectedMsg, got %T", msg)
	}
	if selected.Name != "kanagawa" {
		t.Errorf("selected = %q, want %q", selected.Name, "kanagawa")
	}
}

func TestSkinPicker_JKNavigation(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	// j moves down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	selected := msg.(skinpicker.SkinSelectedMsg)
	if selected.Name != "catppuccin" {
		t.Errorf("selected = %q, want %q", selected.Name, "catppuccin")
	}
}

func TestSkinPicker_KNavigatesUp(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	// j down then k up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	selected := msg.(skinpicker.SkinSelectedMsg)
	if selected.Name != "tokyo-night" {
		t.Errorf("selected = %q, want %q", selected.Name, "tokyo-night")
	}
}

func TestSkinPicker_UpBounds(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	// Up at cursor 0 stays at 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	selected := msg.(skinpicker.SkinSelectedMsg)
	if selected.Name != "tokyo-night" {
		t.Errorf("selected = %q, want %q", selected.Name, "tokyo-night")
	}
}

func TestSkinPicker_DownBounds(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	// Down past end stays at last
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	selected := msg.(skinpicker.SkinSelectedMsg)
	if selected.Name != "catppuccin" {
		t.Errorf("selected = %q, want %q", selected.Name, "catppuccin")
	}
}

func TestSkinPicker_EnterEmpty(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSize(80, 40)
	m.Open()

	// Enter with no skins is a no-op
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd for empty skins")
	}
}

func TestSkinPicker_ViewShowsSkins(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night", "catppuccin"})
	m.SetSize(80, 40)
	m.Open()

	view := m.View()
	if !strings.Contains(view, "Select Theme") {
		t.Errorf("expected 'Select Theme' in view:\n%s", view)
	}
	if !strings.Contains(view, "tokyo-night") {
		t.Errorf("expected 'tokyo-night' in view:\n%s", view)
	}
	if !strings.Contains(view, "catppuccin") {
		t.Errorf("expected 'catppuccin' in view:\n%s", view)
	}
}

func TestSkinPicker_ViewHiddenEmpty(t *testing.T) {
	m := skinpicker.New(testStyles())
	if m.View() != "" {
		t.Error("expected empty view when not visible")
	}
}

func TestSkinPicker_IgnoresInputWhenNotVisible(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night"})

	// Not visible — should not respond
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not produce command when not visible")
	}
}

func TestSkinPicker_FocusBlur(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night"})
	m.SetSize(80, 40)
	m.Open()

	// Blur — should not respond to keys
	m.Blur()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not produce command when blurred")
	}

	// Re-focus — should respond again
	m.Focus()
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected command when re-focused")
	}
}

func TestSkinPicker_SetStyles(t *testing.T) {
	m := skinpicker.New(testStyles())
	m.SetSkins([]string{"tokyo-night"})
	m.SetSize(80, 40)

	newStyles := styles.New(theme.Catppuccin())
	m.SetStyles(newStyles)
	m.Open()

	// Should still render
	view := m.View()
	if !strings.Contains(view, "tokyo-night") {
		t.Errorf("expected 'tokyo-night' in view after SetStyles:\n%s", view)
	}
}
