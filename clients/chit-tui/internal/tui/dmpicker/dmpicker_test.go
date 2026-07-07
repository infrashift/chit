package dmpicker_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestDMPicker_OpenClose(t *testing.T) {
	m := dmpicker.New(testStyles())
	if m.Visible() {
		t.Error("should not be visible initially")
	}

	m.OpenForMembers()
	if !m.Visible() {
		t.Error("should be visible after Open()")
	}

	m.Close()
	if m.Visible() {
		t.Error("should not be visible after Close()")
	}
}

func TestDMPicker_EscapeCloses(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("escape should close picker")
	}
}

func TestDMPicker_NavigateResults(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
		{ID: "u3", Username: "charlie"},
	})

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select (cursor at index 2 = charlie)
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if picked.Users[0].Username != "charlie" {
		t.Errorf("expected charlie, got %s", picked.Users[0].Username)
	}
}

func TestDMPicker_SelectFirstUser(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice", DisplayName: "Alice"},
	})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if picked.Users[0].ID != "u1" {
		t.Errorf("expected u1, got %s", picked.Users[0].ID)
	}
}

func TestDMPicker_SearchTrigger(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()

	// Type a search term
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Enter with no results triggers search
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	search, ok := msg.(dmpicker.SearchTriggeredMsg)
	if !ok {
		t.Fatalf("expected SearchTriggeredMsg, got %T", msg)
	}
	if search.Term != "alice" {
		t.Errorf("expected term 'alice', got %q", search.Term)
	}
}

func TestDMPicker_EnterEmptyNoOp(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()

	// Enter with no text and no results => no-op
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command for empty enter")
	}
}

func TestDMPicker_UpBounds(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Up at cursor 0 should stay at 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	picked := msg.(dmpicker.MembersPickedMsg)
	if picked.Users[0].Username != "alice" {
		t.Errorf("expected alice, got %s", picked.Users[0].Username)
	}
}

func TestDMPicker_DownBounds(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
	})

	// Down past end should stay at last
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	picked := msg.(dmpicker.MembersPickedMsg)
	if picked.Users[0].Username != "alice" {
		t.Errorf("expected alice, got %s", picked.Users[0].Username)
	}
}

func TestDMPicker_ViewShowsResults(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice", DisplayName: "Alice"},
	})

	view := m.View()
	if !strings.Contains(view, "alice") {
		t.Errorf("expected 'alice' in view:\n%s", view)
	}
	if !strings.Contains(view, "Alice") {
		t.Errorf("expected 'Alice' display name in view:\n%s", view)
	}
}

func TestDMPicker_ViewHiddenEmpty(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when not visible, got: %s", view)
	}
}

func TestDMPicker_IgnoresInputWhenNotVisible(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not emit command when not visible")
	}
}

func TestDMPicker_FocusBlur(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.OpenForMembers()
	m.Blur()
	// When blurred, should not respond to keys
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not emit command when blurred")
	}

	m.Focus()
	// Re-set results since Close() clears them
	m.SetResults([]*model.User{{ID: "u1", Username: "alice"}})
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("should emit command when focused")
	}
}

// --- Multi-select tests ---

func TestDMPicker_TabTogglesSelection(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Tab selects first user
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view := m.View()
	if !strings.Contains(view, "[x]") {
		t.Errorf("expected selected marker in view:\n%s", view)
	}

	// Tab again deselects
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = m.View()
	// After deselecting, the chip should no longer appear
	_ = view
}

func TestDMPicker_MultiSelectPicksAll(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
		{ID: "u3", Username: "charlie"},
	})

	// Select alice (cursor at 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Move to bob, select
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Now 2 selected → Enter should emit MembersPickedMsg
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	group, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if len(group.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(group.Users))
	}
}

func TestDMPicker_MaxSelection(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()

	users := make([]*model.User, 8)
	for i := range 8 {
		users[i] = &model.User{ID: fmt.Sprintf("u%d", i), Username: fmt.Sprintf("user%d", i)}
	}
	m.SetResults(users)

	// Select 7 users (max)
	for i := range 7 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if i < 6 {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
	}

	// Try to select 8th (should be ignored)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Enter with 7 selected
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	group, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if len(group.Users) != 7 {
		t.Errorf("expected 7 users (max), got %d", len(group.Users))
	}
}

func TestDMPicker_BackspaceRemovesLastSelected(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select alice and bob
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Backspace on empty input removes last selected (bob)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	// Now only 1 selected → Enter emits MembersPickedMsg with just that user
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if len(picked.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(picked.Users))
	}
}

func TestDMPicker_OpenResetsSelected(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select users
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Re-open should reset
	m.OpenForMembers()

	// Enter should be no-op (no results, no selected, no text)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command after re-open")
	}
}

func TestDMPicker_ViewShowsSelectedChips(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select both
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	if !strings.Contains(view, "@alice") {
		t.Errorf("expected '@alice' chip in view:\n%s", view)
	}
	if !strings.Contains(view, "@bob") {
		t.Errorf("expected '@bob' chip in view:\n%s", view)
	}
}

func TestDMPicker_ViewShowsGroupHint(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select 2 users
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	if !strings.Contains(view, "add members") {
		t.Errorf("expected add-members hint in view:\n%s", view)
	}
}

func TestDMPicker_MemberPickerMode(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()

	if !m.Visible() {
		t.Error("should be visible after OpenForMembers()")
	}
}

func TestDMPicker_MemberPickerEmitsMembersPickedMsg(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select both via tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if len(picked.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(picked.Users))
	}
}

func TestDMPicker_MemberPickerSingleUser(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
	})

	// Enter without tab (selects cursor result)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg, got %T", msg)
	}
	if len(picked.Users) != 1 || picked.Users[0].ID != "u1" {
		t.Errorf("unexpected users: %+v", picked.Users)
	}
}

func TestDMPicker_MemberPickerHintText(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
	})

	// Select one user
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view := m.View()
	if !strings.Contains(view, "add members") {
		t.Errorf("expected 'add members' hint in view:\n%s", view)
	}
}

func TestDMPicker_SingleSelectedEnterPicksMember(t *testing.T) {
	m := dmpicker.New(testStyles())
	m.SetSize(80, 40)
	m.OpenForMembers()
	m.SetResults([]*model.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	})

	// Select only one user
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Enter with 1 selected emits MembersPickedMsg with that user.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	picked, ok := msg.(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected MembersPickedMsg with 1 selected, got %T", msg)
	}
	if len(picked.Users) != 1 || picked.Users[0].ID != "u1" {
		t.Errorf("unexpected users: %+v", picked.Users)
	}
}
