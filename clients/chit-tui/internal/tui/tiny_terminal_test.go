package tui_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
)

// tinySizes are terminal sizes that leave the panes no room. The terminal
// size is input, so no size may fail an assertion (ADR-0011): a panic here
// ends the program for the user.
func tinySizes() []tea.WindowSizeMsg {
	return []tea.WindowSizeMsg{
		{Width: 0, Height: 0}, {Width: 1, Height: 1}, {Width: 10, Height: 2},
		{Width: 20, Height: 5}, {Width: 3, Height: 40}, {Width: 200, Height: 3},
	}
}

// renderAt resizes m, renders it, and opens and renders the help overlay.
// A failed assertion panics, which fails the test.
func renderAt(t *testing.T, m tui.Model, size tea.WindowSizeMsg) {
	t.Helper()
	m, _ = step(t, m, size)
	_ = m.View()
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	_ = m.View()
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.View() == "" {
		t.Fatalf("after %dx%d, the normal size renders nothing", size.Width, size.Height)
	}
}

func TestTinyTerminalSignedIn(t *testing.T) {
	for _, size := range tinySizes() {
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			m, _ := step(t, testModel(), tea.WindowSizeMsg{Width: 120, Height: 40})
			m, _ = step(t, m, tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
			renderAt(t, m, size)
		})
	}
}

func TestTinyTerminalSignIn(t *testing.T) {
	for _, size := range tinySizes() {
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			renderAt(t, testModel(), size)
		})
	}
}
