package tagpicker_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func testTags() []*model.Tag {
	return []*model.Tag{
		{ID: "t1", Name: "urgent"},
		{ID: "t2", Name: "bug"},
		{ID: "t3", Name: "feature"},
	}
}

func TestTagPicker_OpenClose(t *testing.T) {
	m := tagpicker.New(testStyles())
	if m.Visible() {
		t.Error("expected not visible initially")
	}

	m.Open("p1", testTags(), nil)
	if !m.Visible() {
		t.Error("expected visible after open")
	}

	m.Close()
	if m.Visible() {
		t.Error("expected not visible after close")
	}
}

func TestTagPicker_ViewShowsTags(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)
	view := m.View()
	if !strings.Contains(view, "urgent") {
		t.Errorf("expected 'urgent' in view:\n%s", view)
	}
	if !strings.Contains(view, "bug") {
		t.Errorf("expected 'bug' in view:\n%s", view)
	}
}

func TestTagPicker_ViewShowsApplied(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	applied := []*model.Tag{{ID: "t1", Name: "urgent"}}
	m.Open("p1", testTags(), applied)
	view := m.View()
	if !strings.Contains(view, "[x]") {
		t.Errorf("expected '[x]' for applied tag in view:\n%s", view)
	}
}

func TestTagPicker_EscapeCloses(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("expected not visible after escape")
	}
}

func TestTagPicker_EnterTogglesTag(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	toggled, ok := msg.(tagpicker.TagToggledMsg)
	if !ok {
		t.Fatalf("expected TagToggledMsg, got %T", msg)
	}
	if toggled.PostID != "p1" {
		t.Errorf("postID = %q", toggled.PostID)
	}
	if !toggled.Applied {
		t.Error("expected Applied=true for first toggle")
	}
}

func TestTagPicker_CursorNavigation(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Select second item
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	toggled := msg.(tagpicker.TagToggledMsg)
	if toggled.TagID != "t2" {
		t.Errorf("expected t2, got %q", toggled.TagID)
	}
}

func TestTagPicker_CtrlNCreateTag(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Type a name
	for _, r := range "newt" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if cmd == nil {
		t.Fatal("expected command from ctrl+n")
	}
	msg := cmd()
	created, ok := msg.(tagpicker.TagCreateRequestMsg)
	if !ok {
		t.Fatalf("expected TagCreateRequestMsg, got %T", msg)
	}
	if created.Name != "newt" {
		t.Errorf("name = %q", created.Name)
	}
	if created.PostID != "p1" {
		t.Errorf("postID = %q", created.PostID)
	}
}

func TestTagPicker_NotVisibleReturnsEmpty(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	if m.View() != "" {
		t.Error("expected empty view when not visible")
	}
}

func TestTagPicker_UpdateWhenNotVisible(t *testing.T) {
	m := tagpicker.New(testStyles())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when not visible")
	}
}

func TestTagPicker_CtrlNEmptyInput(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if cmd != nil {
		t.Error("expected nil command for ctrl+n with empty input")
	}
}

func TestTagPicker_CursorUpAtTop(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Already at top, up should be no-op
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	toggled := msg.(tagpicker.TagToggledMsg)
	if toggled.TagID != "t1" {
		t.Errorf("expected t1 (first), got %q", toggled.TagID)
	}
}

func TestTagPicker_CursorDownAtBottom(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Move past end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // past end
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	toggled := msg.(tagpicker.TagToggledMsg)
	if toggled.TagID != "t3" {
		t.Errorf("expected t3 (last), got %q", toggled.TagID)
	}
}

func TestTagPicker_FilterNoMatches(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Type something that matches nothing
	for _, r := range "zzz" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := m.View()
	if !strings.Contains(view, "Ctrl+N") {
		t.Errorf("expected 'Ctrl+N' hint when no matches:\n%s", view)
	}
}

func TestTagPicker_EnterWithNoFiltered(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	m.Open("p1", testTags(), nil)

	// Filter to nothing
	for _, r := range "zzz" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Enter with no filtered items
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when no filtered items")
	}
}

func TestTagPicker_ToggleOffAppliedTag(t *testing.T) {
	m := tagpicker.New(testStyles())
	m.SetSize(80, 24)
	applied := []*model.Tag{{ID: "t1", Name: "urgent"}}
	m.Open("p1", testTags(), applied)

	// Toggle off the applied tag
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	toggled := msg.(tagpicker.TagToggledMsg)
	if toggled.Applied {
		t.Error("expected Applied=false for toggling off")
	}
	if toggled.TagID != "t1" {
		t.Errorf("expected t1, got %q", toggled.TagID)
	}
}
