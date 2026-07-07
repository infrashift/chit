package search_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/model"
	"github.com/infrashift/chit-tui/internal/tui/search"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

func newTestSearch() search.Model {
	s := styles.New(theme.TokyoNight())
	m := search.New(s)
	m.SetSize(100, 40)
	m.Open()
	return m
}

func TestSearch_OpenClose(t *testing.T) {
	m := newTestSearch()
	if !m.Visible() {
		t.Error("expected visible after Open")
	}

	m.Close()
	if m.Visible() {
		t.Error("expected not visible after Close")
	}
}

func TestSearch_EscCloses(t *testing.T) {
	m := newTestSearch()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("expected Esc to close search")
	}
}

func TestSearch_EnterSubmitsSearch(t *testing.T) {
	m := newTestSearch()

	// Type a query
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})

	// Press Enter to submit
	var cmd tea.Cmd
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected SubmitMsg command")
	}
	msg := cmd()
	sub, ok := msg.(search.SubmitMsg)
	if !ok {
		t.Fatalf("expected SubmitMsg, got %T", msg)
	}
	if sub.Term != "hello" {
		t.Errorf("term = %q, want %q", sub.Term, "hello")
	}
}

func TestSearch_ResultSelection(t *testing.T) {
	m := newTestSearch()
	m.SetResults([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "first match"},
		{ID: "p2", UserID: "u1", Content: "second match"},
	})

	// Navigate down to second result
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select it
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected ResultSelectedMsg command")
	}
	msg := cmd()
	sel, ok := msg.(search.ResultSelectedMsg)
	if !ok {
		t.Fatalf("expected ResultSelectedMsg, got %T", msg)
	}
	if sel.Post.ID != "p2" {
		t.Errorf("selected post ID = %q, want %q", sel.Post.ID, "p2")
	}
}

func TestSearch_ViewShowsResults(t *testing.T) {
	m := newTestSearch()
	m.SetResults([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "match content"},
	})

	view := m.View()
	if !strings.Contains(view, "match content") {
		t.Errorf("expected 'match content' in view:\n%s", view)
	}
}

func TestSearch_ViewHiddenWhenClosed(t *testing.T) {
	s := styles.New(theme.TokyoNight())
	m := search.New(s)
	if m.View() != "" {
		t.Error("expected empty view when not visible")
	}
}

func TestSearch_UpDownNavigation(t *testing.T) {
	m := newTestSearch()
	m.SetResults([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "first"},
		{ID: "p2", UserID: "u1", Content: "second"},
		{ID: "p3", UserID: "u1", Content: "third"},
	})

	// Down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Up once
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Select — should be second result
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	sel := msg.(search.ResultSelectedMsg)
	if sel.Post.ID != "p2" {
		t.Errorf("selected post ID = %q, want %q", sel.Post.ID, "p2")
	}
}

func TestSearch_BlurIgnoresKeys(t *testing.T) {
	m := newTestSearch()
	m.Blur()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test")})
	// Should not crash or modify state when blurred
}

func TestSearch_SetError(t *testing.T) {
	m := newTestSearch()
	m.SetError("connection refused")
	view := m.View()
	if !strings.Contains(view, "connection refused") {
		t.Errorf("expected error in view, got:\n%s", view)
	}
	// Should NOT show "Press Enter to search" when error is set
	if strings.Contains(view, "Press Enter to search") {
		t.Error("should not show 'Press Enter to search' when error is displayed")
	}
}

func TestSearch_ErrorClearedOnResults(t *testing.T) {
	m := newTestSearch()
	m.SetError("some error")
	m.SetResults([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "result"},
	})
	view := m.View()
	if strings.Contains(view, "some error") {
		t.Error("error should be cleared after SetResults")
	}
	if !strings.Contains(view, "result") {
		t.Error("expected results in view")
	}
}

func TestSearch_ErrorClearedOnOpen(t *testing.T) {
	m := newTestSearch()
	m.SetError("old error")
	m.Open()
	view := m.View()
	if strings.Contains(view, "old error") {
		t.Error("error should be cleared after Open")
	}
}

func TestSearch_NoMatchesFound(t *testing.T) {
	m := newTestSearch()
	// Type a query
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nonexistent")})
	// Submit search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Simulate empty results
	m.SetResults([]*model.Post{})

	view := m.View()
	if !strings.Contains(view, "No matches found") {
		t.Errorf("expected 'No matches found' in view:\n%s", view)
	}
}

func TestSearch_PressEnterToSearch(t *testing.T) {
	m := newTestSearch()
	// Type a query but don't submit
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("query")})

	view := m.View()
	if !strings.Contains(view, "Press Enter to search") {
		t.Errorf("expected 'Press Enter to search' in view:\n%s", view)
	}
}

func TestSearch_EditAfterNoMatches(t *testing.T) {
	m := newTestSearch()
	// Type and submit
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("query")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.SetResults([]*model.Post{})

	// Verify "No matches found"
	view := m.View()
	if !strings.Contains(view, "No matches found") {
		t.Fatalf("expected 'No matches found' before edit, got:\n%s", view)
	}

	// Type more — should reset to "Press Enter to search"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	view = m.View()
	if !strings.Contains(view, "Press Enter to search") {
		t.Errorf("expected 'Press Enter to search' after editing query, got:\n%s", view)
	}
	if strings.Contains(view, "No matches found") {
		t.Error("should not show 'No matches found' after editing query")
	}
}
