package viewport_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestViewport_SetPosts(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u2", Content: "World", CreateAt: 1700000001000},
	})

	if len(m.Posts()) != 2 {
		t.Errorf("expected 2 posts, got %d", len(m.Posts()))
	}

	view := m.View()
	if !strings.Contains(view, "Hello") || !strings.Contains(view, "World") {
		t.Errorf("expected posts in view:\n%s", view)
	}
}

func TestViewport_AppendPost(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
	})

	m.AppendPost(&model.Post{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000})

	if len(m.Posts()) != 2 {
		t.Errorf("expected 2 posts, got %d", len(m.Posts()))
	}
}

func TestViewport_AppendPostKeepsExistingSelection(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.Focus()
	// SetPosts takes server order (newest first) and reverses for display.
	m.SetPosts([]*model.Post{
		{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000},
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
	})
	// Move the selection off the last post (p2 at the bottom) up to p1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	m.AppendPost(&model.Post{ID: "p3", UserID: "u1", Content: "Third", CreateAt: 1700000002000})

	if p := m.SelectedPost(); p == nil || p.ID != "p1" {
		t.Errorf("expected selection to stay on p1, got %v", p)
	}
}

func TestViewport_AppendPostIntoEmptyChannelSelectsIt(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// A channel that starts empty leaves the cursor unset (-1).
	m.SetPosts(nil)

	m.AppendPost(&model.Post{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000})

	if p := m.SelectedPost(); p == nil || p.ID != "p1" {
		t.Errorf("expected the first appended post to be selected, got %v", p)
	}
}

func TestViewport_SetUsernames(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	m.SetUsernames(map[string]string{"u1": "alice"})

	view := m.View()
	if !strings.Contains(view, "alice") {
		t.Errorf("expected 'alice' in view:\n%s", view)
	}
}

func TestViewport_SelectedPost(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	p := m.SelectedPost()
	if p == nil || p.ID != "p1" {
		t.Errorf("unexpected selected post: %+v", p)
	}
}

func TestViewport_SelectPostMsg(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	m.Focus()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	sel, ok := msg.(viewport.PostSelectedMsg)
	if !ok {
		t.Fatalf("expected PostSelectedMsg, got %T", msg)
	}
	if sel.Post.ID != "p1" {
		t.Errorf("selected post ID = %q", sel.Post.ID)
	}
}

func TestViewport_ChannelSwitch(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "old", CreateAt: 1700000000000},
	})

	// Switch channel
	m.SetPosts([]*model.Post{
		{ID: "p2", UserID: "u2", Content: "new", CreateAt: 1700000000000},
	})

	if len(m.Posts()) != 1 || m.Posts()[0].ID != "p2" {
		t.Errorf("expected new channel posts")
	}
}

func TestViewport_SetCurrentUsername(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "hello @bob", CreateAt: 1700000000000},
	})
	m.SetUsernames(map[string]string{"u1": "alice"})

	// Set current username — should clear cache and re-render
	m.SetCurrentUsername("bob")

	view := m.View()
	if !strings.Contains(view, "@bob") {
		t.Errorf("expected '@bob' in view after SetCurrentUsername:\n%s", view)
	}
}

func TestViewport_UpdateNotFocused(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// Not focused — should return nil cmd
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = updated
	if cmd != nil {
		t.Error("expected nil cmd when not focused")
	}
}

func TestViewport_UpdateNonKeyMsg(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	m.Focus()

	// Non-key message when focused — delegates to inner viewport
	updated, _ := m.Update(tea.MouseMsg{})
	_ = updated
}

func TestViewport_UpdateDefaultKey(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	m.Focus()

	// Non-enter key when focused — delegates to inner viewport
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = updated
}

func TestViewport_ViewFocused(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.Focus()
	view := m.View()
	if view == "" {
		t.Error("expected non-empty focused view")
	}
}

func TestViewport_SelectedPostEmpty(t *testing.T) {
	m := viewport.New(testStyles())
	p := m.SelectedPost()
	if p != nil {
		t.Errorf("expected nil for empty post list, got %+v", p)
	}
}

func TestViewport_SetThreadCounts(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	m.SetUsernames(map[string]string{"u1": "alice"})

	m.SetThreadCounts(map[string]int{"p1": 5})

	view := m.View()
	if !strings.Contains(view, "5 replies") {
		t.Errorf("expected '5 replies' badge in view:\n%s", view)
	}
}

func TestViewport_SetThreadCountsCacheInvalidation(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u1", Content: "World", CreateAt: 1700000001000},
	})
	m.SetUsernames(map[string]string{"u1": "alice"})

	// Set initial counts
	m.SetThreadCounts(map[string]int{"p1": 2})
	view := m.View()
	if !strings.Contains(view, "2 replies") {
		t.Errorf("expected '2 replies' badge:\n%s", view)
	}

	// Update count for p1, p2 unchanged (should only invalidate p1 cache)
	m.SetThreadCounts(map[string]int{"p1": 5})
	view = m.View()
	if !strings.Contains(view, "5 replies") {
		t.Errorf("expected '5 replies' badge after update:\n%s", view)
	}
}

func TestViewport_CursorMovement(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// Input is API order (newest first). SetPosts reverses to [p3, p2, p1]
	// so display order top→bottom is: p3, p2, p1. Cursor starts at last index (p1).
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000},
		{ID: "p3", UserID: "u3", Content: "Third", CreateAt: 1700000002000},
	})
	m.Focus()

	// Cursor starts at the last internal index (p1, displayed at bottom)
	if p := m.SelectedPost(); p == nil || p.ID != "p1" {
		t.Fatalf("expected cursor at p1, got %v", p)
	}

	// Move up with k — goes to p2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p := m.SelectedPost(); p == nil || p.ID != "p2" {
		t.Fatalf("expected cursor at p2 after k, got %v", p)
	}

	// Move up again — goes to p3 (top)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p := m.SelectedPost(); p == nil || p.ID != "p3" {
		t.Fatalf("expected cursor at p3 after k, got %v", p)
	}

	// Move up at top — should stay at p3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p := m.SelectedPost(); p == nil || p.ID != "p3" {
		t.Fatalf("expected cursor to stay at p3, got %v", p)
	}

	// Move down with j — goes to p2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p := m.SelectedPost(); p == nil || p.ID != "p2" {
		t.Fatalf("expected cursor at p2 after j, got %v", p)
	}

	// Move to bottom and try past it
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p := m.SelectedPost(); p == nil || p.ID != "p1" {
		t.Fatalf("expected cursor to stay at p1, got %v", p)
	}
}

func TestViewport_CursorEnterSelectsPost(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// After reversal: [p2, p1], cursor starts at index 1 (p1)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000},
	})
	m.Focus()

	// Move cursor up to p2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	// Press enter — should select p2
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	sel, ok := msg.(viewport.PostSelectedMsg)
	if !ok {
		t.Fatalf("expected PostSelectedMsg, got %T", msg)
	}
	if sel.Post.ID != "p2" {
		t.Errorf("expected p2, got %q", sel.Post.ID)
	}
	_ = m
}

func TestViewport_SelectedPostHighlighted(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000},
	})
	m.SetUsernames(map[string]string{"u1": "alice", "u2": "bob"})
	m.Focus()

	// Cursor is at p2 (last). Move to p1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	view := m.View()
	// Both posts should be visible
	if !strings.Contains(view, "First") || !strings.Contains(view, "Second") {
		t.Errorf("expected both posts in view:\n%s", view)
	}
}

func TestViewport_SetStyles(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000},
	})
	// SetStyles should re-render without panic
	m.SetStyles(styles.New(theme.Catppuccin()))
	view := m.View()
	if !strings.Contains(view, "Hello") {
		t.Errorf("expected 'Hello' after SetStyles:\n%s", view)
	}
}

func TestViewport_CursorUpArrow(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u2", Content: "Second", CreateAt: 1700000001000},
	})
	m.Focus()

	// Up arrow moves cursor up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p := m.SelectedPost(); p == nil || p.ID != "p2" {
		t.Fatalf("expected cursor at p2 after up, got %v", p)
	}

	// Down arrow moves cursor back down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p := m.SelectedPost(); p == nil || p.ID != "p1" {
		t.Fatalf("expected cursor at p1 after down, got %v", p)
	}
}

func TestViewport_DaySeparator_MultiDay(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// Two posts on different days (86400000ms = 1 day apart).
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Day one", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u1", Content: "Day two", CreateAt: 1700000000000 + 86400000},
	})

	view := m.View()
	if !strings.Contains(view, "─") {
		t.Errorf("expected day separator in view:\n%s", view)
	}
}

func TestViewport_DaySeparator_SameDay(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	// Two posts on the same day (1 second apart).
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "First", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u1", Content: "Second", CreateAt: 1700000001000},
	})

	view := m.View()
	// The rounded border uses ─ so we check for the day separator label pattern.
	if strings.Contains(view, "2023-11") {
		t.Errorf("did not expect a date label separator for same-day posts:\n%s", view)
	}
}

func TestViewport_DaySeparator_SinglePost(t *testing.T) {
	m := viewport.New(testStyles())
	m.SetSize(80, 24)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "Only one", CreateAt: 1700000000000},
	})

	view := m.View()
	// No separator should appear with a single post.
	if strings.Contains(view, "2023-11") {
		t.Errorf("did not expect a date separator for a single post:\n%s", view)
	}
}

func TestViewport_FocusBlur(t *testing.T) {
	m := viewport.New(testStyles())
	if m.Focused() {
		t.Error("should not be focused initially")
	}
	m.Focus()
	if !m.Focused() {
		t.Error("should be focused")
	}
	m.Blur()
	if m.Focused() {
		t.Error("should not be focused after blur")
	}
}

func TestScrollToPost(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "first", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u1", Content: "second", CreateAt: 1700000001000},
		{ID: "p3", UserID: "u1", Content: "third", CreateAt: 1700000002000},
	})

	if !m.ScrollToPost("p3") {
		t.Fatal("ScrollToPost returned false for a loaded post")
	}
	if got := m.SelectedPost(); got == nil || got.ID != "p3" {
		t.Errorf("SelectedPost = %v, want p3", got)
	}
}

// A search can return a hit older than the loaded window; the caller needs to
// know so it can say so rather than appearing to do nothing.
func TestScrollToPostReportsMissing(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{{ID: "p1", UserID: "u1", Content: "only", CreateAt: 1700000000000}})

	if m.ScrollToPost("nope") {
		t.Error("ScrollToPost returned true for a post that is not loaded")
	}
}

func TestSearchTermHighlighting(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "the quick brown fox", CreateAt: 1700000000000},
	})

	plain := testutil.StripANSI(m.View())
	m.SetSearchTerm("quick")

	if m.SearchTerm() != "quick" {
		t.Errorf("SearchTerm = %q", m.SearchTerm())
	}
	// The visible text must be unchanged — only its styling differs.
	if got := testutil.StripANSI(m.View()); got != plain {
		t.Errorf("highlighting altered the text:\n%s\nwant:\n%s", got, plain)
	}
	// And the styled output must actually differ.
	if m.View() == plain {
		t.Error("no styling was applied for the search term")
	}
}

// Highlighting operates on rendered text containing ANSI escapes. A term that
// appears inside an escape sequence must not be matched, or the output is
// corrupted.
func TestSearchTermDoesNotMatchInsideEscapes(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "hello world", CreateAt: 1700000000000},
	})

	before := testutil.StripANSI(m.View())
	// "m" terminates every SGR sequence, so a naive matcher corrupts them.
	m.SetSearchTerm("m")

	if got := testutil.StripANSI(m.View()); got != before {
		t.Errorf("matching inside escape sequences corrupted the output:\n%s", got)
	}
}

func TestSetSearchTermClearing(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "alpha beta", CreateAt: 1700000000000},
	})

	original := m.View()
	m.SetSearchTerm("alpha")
	m.SetSearchTerm("")

	if m.View() != original {
		t.Error("clearing the search term did not restore the original rendering")
	}
}

func newSelectionModel(t *testing.T) viewport.Model {
	t.Helper()
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 20)
	m.SetPosts([]*model.Post{
		{ID: "p1", UserID: "u1", Content: "alpha", CreateAt: 1700000000000},
		{ID: "p2", UserID: "u1", Content: "beta", CreateAt: 1700000001000},
	})
	return m
}

func TestSelectionLifecycle(t *testing.T) {
	m := newSelectionModel(t)

	if m.HasSelection() {
		t.Error("a fresh viewport should have no selection")
	}
	if m.SelectedText() != "" {
		t.Errorf("SelectedText = %q, want empty", m.SelectedText())
	}

	m.SetSelectionAnchor(0)
	if !m.HasSelection() {
		t.Fatal("HasSelection is false after anchoring")
	}
	if m.SelectedText() == "" {
		t.Error("SelectedText is empty after anchoring")
	}

	m.ClearSelection()
	if m.HasSelection() {
		t.Error("HasSelection is true after clearing")
	}
}

// Dragging upwards puts the head before the anchor; the range must still be
// ordered or the slice bounds are inverted.
func TestSelectionHandlesUpwardDrag(t *testing.T) {
	m := newSelectionModel(t)

	m.SetSelectionAnchor(3)
	m.ExtendSelection(0)

	got := m.SelectedText()
	if got == "" {
		t.Fatal("SelectedText is empty after an upward drag")
	}
	if strings.Count(got, "\n") < 1 {
		t.Errorf("expected a multi-line selection, got %q", got)
	}
}

// Copied text goes to a clipboard, so it must carry no escape sequences.
func TestSelectedTextIsPlain(t *testing.T) {
	m := newSelectionModel(t)
	m.SetSelectionAnchor(0)
	m.ExtendSelection(5)

	got := m.SelectedText()
	if got == "" {
		t.Fatal("SelectedText is empty")
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("selected text contains escape sequences: %q", got)
	}
}

func TestExtendSelectionWithoutAnchorIsNoop(t *testing.T) {
	m := newSelectionModel(t)

	m.ExtendSelection(3)
	if m.HasSelection() {
		t.Error("ExtendSelection started a selection without an anchor")
	}
}

// Out-of-range rows come from clicks below the last post; they must clamp
// rather than panic.
func TestSelectionClampsOutOfRange(t *testing.T) {
	m := newSelectionModel(t)

	m.SetSelectionAnchor(-5)
	m.ExtendSelection(9999)

	if m.SelectedText() == "" {
		t.Error("clamped selection produced no text")
	}
}

// The reverse line map is what makes click-to-select possible; post→line
// offsets alone cannot answer "which post is at this row".
func TestSelectPostAtLine(t *testing.T) {
	m := newSelectionModel(t)

	// SetPosts reverses its input (the API returns newest first), so the
	// first rendered post is the last one passed in.
	if !m.SelectPostAtLine(0) {
		t.Fatal("SelectPostAtLine(0) found no post")
	}
	first := m.SelectedPost()
	if first == nil {
		t.Fatal("SelectedPost is nil after selecting line 0")
	}

	// A row inside the second rendered post must resolve to a different post.
	if !m.SelectPostAtLine(4) {
		t.Fatal("SelectPostAtLine(4) found no post")
	}
	second := m.SelectedPost()
	if second == nil || second.ID == first.ID {
		t.Errorf("rows 0 and 4 resolved to the same post (%v); the line map is wrong", first)
	}

	if m.SelectPostAtLine(9999) {
		t.Error("SelectPostAtLine returned true for a row past the content")
	}
}

func TestLineAtAccountsForScroll(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 10)

	// Enough posts that the content actually overflows the pane; scrolling is
	// clamped to zero otherwise and the test would prove nothing.
	posts := make([]*model.Post, 0, 20)
	for i := range 20 {
		posts = append(posts, &model.Post{
			ID: fmt.Sprintf("p%d", i), UserID: "u1",
			Content: fmt.Sprintf("message %d", i), CreateAt: 1700000000000,
		})
	}
	m.SetPosts(posts)

	m.ScrollBy(-9999) // SetPosts lands at the bottom; go back to the top.
	if got := m.LineAt(2); got != 2 {
		t.Fatalf("LineAt(2) = %d, want 2 at the top", got)
	}

	m.ScrollBy(3)
	if got := m.LineAt(2); got != 5 {
		t.Errorf("LineAt(2) = %d, want 5 after scrolling 3", got)
	}
}

// Prepending must keep the reader where they are: the content above them
// grew, so their position has to move down by the same amount.
func TestPrependPostsHoldsScrollPosition(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 10)

	recent := make([]*model.Post, 0, 20)
	for i := range 20 {
		recent = append(recent, &model.Post{
			ID: fmt.Sprintf("r%d", i), UserID: "u1",
			Content: fmt.Sprintf("recent %d", i), CreateAt: 1700000000000,
		})
	}
	m.SetPosts(recent)
	m.ScrollBy(-9999) // to the top of what is loaded

	// Whatever line the reader is looking at must stay put, regardless of
	// which post it belongs to.
	firstLineBefore := strings.SplitN(testutil.StripANSI(m.View()), "\n", 2)[0]

	older := []*model.Post{
		{ID: "o1", UserID: "u1", Content: "older one", CreateAt: 1600000001000},
		{ID: "o2", UserID: "u1", Content: "older two", CreateAt: 1600000000000},
	}
	m.PrependPosts(older)

	firstLineAfter := strings.SplitN(testutil.StripANSI(m.View()), "\n", 2)[0]
	if firstLineAfter != firstLineBefore {
		t.Errorf("the reader's position moved:\nbefore %q\nafter  %q",
			firstLineBefore, firstLineAfter)
	}
	if !m.HasPost("o1") || !m.HasPost("o2") {
		t.Error("older posts were not added")
	}
}

func TestPrependPostsIgnoresEmpty(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 10)
	m.SetPosts([]*model.Post{{ID: "p1", UserID: "u1", Content: "only", CreateAt: 1700000000000}})

	before := m.View()
	m.PrependPosts(nil)

	if m.View() != before {
		t.Error("prepending nothing changed the view")
	}
}

func TestAtTop(t *testing.T) {
	m := viewport.New(styles.New(theme.TokyoNight()))
	m.SetSize(80, 5)

	posts := make([]*model.Post, 0, 30)
	for i := range 30 {
		posts = append(posts, &model.Post{
			ID: fmt.Sprintf("p%d", i), UserID: "u1",
			Content: fmt.Sprintf("message %d", i), CreateAt: 1700000000000,
		})
	}
	m.SetPosts(posts) // lands at the bottom

	if m.AtTop() {
		t.Error("AtTop is true at the bottom of a long history")
	}

	m.ScrollBy(-9999)
	if !m.AtTop() {
		t.Error("AtTop is false after scrolling to the top")
	}
}
