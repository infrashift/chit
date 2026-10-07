package thread_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/thread"
)

func longThread(t *testing.T) thread.Model {
	t.Helper()
	m := thread.New(testutil.Styles())
	m.SetSize(60, 12)
	var replies []*model.Post
	for i := range 20 {
		replies = append(replies, &model.Post{ID: fmt.Sprintf("r%d", i), UserID: "u1", RootID: "root",
			Content: fmt.Sprintf("reply %d", i), CreateAt: int64(1700000001000 + i)})
	}
	m.SetThread(&model.Post{ID: "root", UserID: "u1", Content: "the root", CreateAt: 1700000000000}, replies)
	return m
}

func TestThread_UpdatePostShowsTheEdit(t *testing.T) {
	m := thread.New(testutil.Styles())
	m.SetSize(60, 20)
	m.SetThread(&model.Post{ID: "root", UserID: "u1", Content: "the root", CreateAt: 1700000000000},
		[]*model.Post{{ID: "r1", UserID: "u1", RootID: "root", Content: "a reply", CreateAt: 1700000001000}})

	if !m.UpdatePost(&model.Post{ID: "root", UserID: "u1", Content: "the root, edited", CreateAt: 1700000000000}) {
		t.Fatal("root not found")
	}
	if !m.UpdatePost(&model.Post{ID: "r1", UserID: "u1", RootID: "root", Content: "a reply, edited", CreateAt: 1700000001000}) {
		t.Fatal("reply not found")
	}
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "the root, edited") || !strings.Contains(view, "a reply, edited") {
		t.Errorf("edits not shown:\n%s", view)
	}
	if m.UpdatePost(&model.Post{ID: "elsewhere"}) {
		t.Error("found a post not in the thread")
	}
}

// Keys scroll the thread only while it has focus.
func TestThread_KeysScrollOnlyWhenFocused(t *testing.T) {
	m := longThread(t)
	m.ScrollBy(-1000) // to the top
	top := m.View()
	pgdn := tea.KeyMsg{Type: tea.KeyPgDown}

	m, _ = m.Update(pgdn)
	if m.View() != top {
		t.Error("an unfocused thread scrolled")
	}

	m.Focus()
	m, _ = m.Update(pgdn)
	if m.View() == top {
		t.Error("page down did not scroll a focused thread")
	}
}

// The wheel scrolls the pane whether or not it has focus.
func TestThread_ScrollBy(t *testing.T) {
	m := longThread(t)
	m.ScrollBy(-1000)
	top := m.View()
	m.ScrollBy(5)
	if m.View() == top {
		t.Error("ScrollBy did not move the view")
	}
}
