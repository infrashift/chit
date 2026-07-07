package cmdpalette_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/cmdpalette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func testCommands() []*model.Command {
	return []*model.Command{
		{ID: "c1", Slug: "remind", Description: "Set a reminder", Category: "workflow"},
		{ID: "c2", Slug: "mute", Description: "Mute a channel", Category: "channels"},
		{ID: "c3", Slug: "search", Description: "Search messages", Category: "search"},
	}
}

func TestCmdPalette_OpenClose(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())

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

func TestCmdPalette_SelectCommand(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())
	m.Open()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	sel, ok := msg.(cmdpalette.CommandSelectedMsg)
	if !ok {
		t.Fatalf("expected CommandSelectedMsg, got %T", msg)
	}
	if sel.Command.Slug != "remind" {
		t.Errorf("selected command = %q, want 'remind'", sel.Command.Slug)
	}
}

func TestCmdPalette_Filter(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())
	m.Open()

	// Type "mu" to filter
	for _, ch := range "mu" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Select first filtered item
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	sel, ok := msg.(cmdpalette.CommandSelectedMsg)
	if !ok {
		t.Fatalf("expected CommandSelectedMsg, got %T", msg)
	}
	if sel.Command.Slug != "mute" {
		t.Errorf("filtered command = %q, want 'mute'", sel.Command.Slug)
	}
}

func TestCmdPalette_EscapeCloses(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())
	m.Open()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("should be hidden after escape")
	}
}

func TestCmdPalette_NavigateUpDown(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())
	m.Open()

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	sel := msg.(cmdpalette.CommandSelectedMsg)
	if sel.Command.Slug != "search" {
		t.Errorf("selected = %q, want 'search'", sel.Command.Slug)
	}
}

func TestCmdPalette_ViewContainsCommands(t *testing.T) {
	m := cmdpalette.New(testStyles())
	m.SetSize(80, 24)
	m.SetCommands(testCommands())
	m.Open()

	view := m.View()
	if !strings.Contains(view, "remind") {
		t.Errorf("expected 'remind' in view:\n%s", view)
	}
}

func TestCmdPalette_HiddenViewEmpty(t *testing.T) {
	m := cmdpalette.New(testStyles())
	if m.View() != "" {
		t.Error("expected empty view when hidden")
	}
}
