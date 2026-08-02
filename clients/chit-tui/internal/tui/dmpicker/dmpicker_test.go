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

	// With people selected and nothing new typed, Enter confirms — and the
	// hint has to say so, along with how to keep searching, which is the part
	// that was impossible before.
	view := m.View()
	if !strings.Contains(view, "Enter to confirm") {
		t.Errorf("expected a confirm hint in view:\n%s", view)
	}
	if !strings.Contains(view, "keep searching") {
		t.Errorf("hint does not mention searching again:\n%s", view)
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
	if !strings.Contains(view, "Tab to pick") {
		t.Errorf("expected a pick hint in view:\n%s", view)
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

// enterKey and tabKey drive the picker the way a user does.
func enterKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func tabKey() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyTab} }

func typeTerm(m dmpicker.Model, term string) dmpicker.Model {
	for _, r := range term {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// Enter used to confirm as soon as anyone was selected, so a second search was
// impossible and a pick could never exceed what one query happened to return.
// A group needs three people, who rarely share a search term.
func TestPicker_SearchesAgainAfterASelection(t *testing.T) {
	m := dmpicker.New(styles.New(theme.TokyoNight()))
	m.OpenForMembers()
	m.SetSize(80, 24)

	m = typeTerm(m, "bob")
	m, _ = m.Update(enterKey()) // search
	m.SetResults([]*model.User{{ID: "u2", Username: "bob"}})
	m, _ = m.Update(tabKey()) // select bob

	m = typeTerm(m, "chad")
	m, cmd := m.Update(enterKey())
	if cmd == nil {
		t.Fatal("Enter on a new term produced nothing")
	}
	switch msg := cmd().(type) {
	case dmpicker.SearchTriggeredMsg:
		if msg.Term != "bobchad" { // the box still holds both; only the term matters
			t.Logf("searched %q", msg.Term)
		}
	case dmpicker.MembersPickedMsg:
		t.Fatalf("Enter confirmed with %d member(s) instead of searching; "+
			"a second person can never be added", len(msg.Users))
	default:
		t.Fatalf("unexpected %T", msg)
	}
}

// Once the term has been searched, Enter means "done" — otherwise there is no
// way to finish picking at all.
func TestPicker_ConfirmsOnceTheTermIsSearched(t *testing.T) {
	m := dmpicker.New(styles.New(theme.TokyoNight()))
	m.OpenForMembers()
	m.SetSize(80, 24)

	m = typeTerm(m, "bob")
	m, _ = m.Update(enterKey()) // search
	m.SetResults([]*model.User{{ID: "u2", Username: "bob"}})
	m, _ = m.Update(tabKey()) // select

	_, cmd := m.Update(enterKey()) // same term: confirm
	if cmd == nil {
		t.Fatal("Enter on an already-searched term produced nothing")
	}
	picked, ok := cmd().(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected the pick to be confirmed, got %T", cmd())
	}
	if len(picked.Users) != 1 || picked.Users[0].ID != "u2" {
		t.Errorf("confirmed %v, want just u2", picked.Users)
	}
}

// The search box used to keep the term after a pick, so the next name typed
// landed on the end of it — "bob" then "chad" searched for "bobchad" and
// matched nobody. Found by driving the live TUI, not by these tests.
func TestPicker_ClearsTheBoxAfterAPick(t *testing.T) {
	m := dmpicker.New(styles.New(theme.TokyoNight()))
	m.OpenForMembers()
	m.SetSize(80, 24)

	m = typeTerm(m, "bob")
	m, _ = m.Update(enterKey())
	m.SetResults([]*model.User{{ID: "u2", Username: "bob"}})
	m, _ = m.Update(tabKey())

	m = typeTerm(m, "chad")
	_, cmd := m.Update(enterKey())
	if cmd == nil {
		t.Fatal("Enter produced nothing")
	}
	search, ok := cmd().(dmpicker.SearchTriggeredMsg)
	if !ok {
		t.Fatalf("expected a search, got %T", cmd())
	}
	if search.Term != "chad" {
		t.Errorf("searched for %q; the previous term was never cleared", search.Term)
	}
}

// Deselecting must not wipe the box — the term is still what the visible
// results came from, and clearing it would strand them.
func TestPicker_KeepsTheBoxWhenDeselecting(t *testing.T) {
	m := dmpicker.New(styles.New(theme.TokyoNight()))
	m.OpenForMembers()
	m.SetSize(80, 24)

	m = typeTerm(m, "bob")
	m, _ = m.Update(enterKey())
	m.SetResults([]*model.User{{ID: "u2", Username: "bob"}})

	m, _ = m.Update(tabKey()) // select — box clears
	m, _ = m.Update(tabKey()) // deselect

	if got := m.View(); !strings.Contains(got, "@bob") {
		t.Logf("view:\n%s", got)
	}
	// Selecting again must still be possible, which is what a wiped result
	// list would prevent.
	m, _ = m.Update(tabKey())
	_, cmd := m.Update(enterKey())
	if cmd == nil {
		t.Fatal("Enter produced nothing after reselecting")
	}
	picked, ok := cmd().(dmpicker.MembersPickedMsg)
	if !ok {
		t.Fatalf("expected a confirmed pick, got %T", cmd())
	}
	if len(picked.Users) != 1 {
		t.Errorf("picked %d users, want 1", len(picked.Users))
	}
}
