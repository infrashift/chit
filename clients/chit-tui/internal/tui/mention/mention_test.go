package mention

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func testEntries() []MentionEntry {
	return []MentionEntry{
		{Username: "all", DisplayName: "", Special: true},
		{Username: "channel", DisplayName: "", Special: true},
		{Username: "here", DisplayName: "", Special: true},
		{Username: "alice", DisplayName: "Alice A", UserID: "u1"},
		{Username: "bob", DisplayName: "Bob B", UserID: "u2"},
		{Username: "charlie", DisplayName: "Charlie C", UserID: "u3"},
	}
}

func TestModel_ShowHide(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())

	if m.Visible() {
		t.Error("should not be visible initially")
	}

	m.Show("", 0)
	if !m.Visible() {
		t.Error("should be visible after Show")
	}

	m.Hide()
	if m.Visible() {
		t.Error("should not be visible after Hide")
	}
}

func TestModel_FilterNarrows(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	if len(m.filtered) != 6 {
		t.Fatalf("filtered = %d, want 6", len(m.filtered))
	}

	m.UpdateFilter("al")
	if len(m.filtered) != 2 { // "all" (special) + "alice"
		t.Fatalf("filtered after 'al' = %d, want 2", len(m.filtered))
	}
}

func TestModel_UpDown(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", m.cursor)
	}

	// Should not go below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor after extra up = %d, want 0", m.cursor)
	}
}

func TestModel_EnterSelectsUser(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 5)

	// Move to "alice" (index 3, after 3 specials)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	sel, ok := msg.(UserSelectedMsg)
	if !ok {
		t.Fatalf("expected UserSelectedMsg, got %T", msg)
	}
	if sel.Username != "alice" {
		t.Errorf("username = %q, want 'alice'", sel.Username)
	}
	if sel.StartCol != 5 {
		t.Errorf("startCol = %d, want 5", sel.StartCol)
	}
	if m.Visible() {
		t.Error("should be hidden after selection")
	}
}

func TestModel_TabSelectsUser(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("al", 0)

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected command from tab")
	}
	msg := cmd()
	sel, ok := msg.(UserSelectedMsg)
	if !ok {
		t.Fatalf("expected UserSelectedMsg, got %T", msg)
	}
	// First filtered is "all" (special)
	if sel.Username != "all" {
		t.Errorf("username = %q, want 'all'", sel.Username)
	}
}

func TestModel_EscHides(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("should be hidden after esc")
	}
	if cmd != nil {
		t.Error("esc should not emit a command")
	}
}

func TestModel_ViewWhenVisible(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	view := m.View()
	if !strings.Contains(view, "@alice") {
		t.Errorf("expected '@alice' in view:\n%s", view)
	}
	if !strings.Contains(view, "@all") {
		t.Errorf("expected '@all' in view:\n%s", view)
	}
}

func TestModel_ViewWhenHidden(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when hidden, got %q", view)
	}
}

func TestModel_EmptyPrefixShowsAll(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	// Specials first
	if len(m.filtered) != 6 {
		t.Fatalf("filtered = %d, want 6", len(m.filtered))
	}
	if !m.filtered[0].Special {
		t.Error("first entry should be special")
	}
}

func TestModel_DownDoesNotExceedBounds(t *testing.T) {
	m := New(testStyles())
	m.SetEntries([]MentionEntry{{Username: "alice"}})
	m.Show("", 0)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (only 1 entry)", m.cursor)
	}
}

func TestModel_UpdateFilter_ResetsCursor(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	m.Show("", 0)

	// Move cursor down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Filter to 1 item
	m.UpdateFilter("charlie")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after filter narrows", m.cursor)
	}
}

func TestModel_IgnoresWhenHidden(t *testing.T) {
	m := New(testStyles())
	m.SetEntries(testEntries())
	// Not visible

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("should not produce cmd when hidden")
	}
}
