package viewport_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// three is a focused history of a, b, c (oldest to newest), with c selected.
func three(t *testing.T) viewport.Model {
	t.Helper()
	m := viewport.New(testutil.Styles())
	m.SetSize(80, 30)
	m.SetPosts([]*model.Post{ // newest first, as the API returns them
		{ID: "c", UserID: "u1", Content: "third", CreateAt: 1700000003000},
		{ID: "b", UserID: "u1", Content: "second", CreateAt: 1700000002000},
		{ID: "a", UserID: "u1", Content: "first", CreateAt: 1700000001000},
	})
	m.Focus()
	return m
}

func plain(m viewport.Model) string { return testutil.StripANSI(m.View()) }

func TestViewport_UpdatePostShowsTheEdit(t *testing.T) {
	m := three(t)
	if !m.UpdatePost(&model.Post{ID: "b", UserID: "u1", Content: "second, edited", CreateAt: 1700000002000}) {
		t.Fatal("UpdatePost did not find b")
	}
	if !strings.Contains(plain(m), "second, edited") {
		t.Errorf("edit not shown:\n%s", plain(m))
	}
	if m.UpdatePost(&model.Post{ID: "nope"}) {
		t.Error("UpdatePost found a post that is not loaded")
	}
}

func TestViewport_SetPinnedShowsTheBadge(t *testing.T) {
	m := three(t)
	m.SetPinned("b", true)
	if !strings.Contains(plain(m), "[pinned]") {
		t.Errorf("pin badge not shown:\n%s", plain(m))
	}
	m.SetPinned("b", false)
	if strings.Contains(plain(m), "[pinned]") {
		t.Errorf("pin badge still shown after unpinning:\n%s", plain(m))
	}
}

// Removing a post above the cursor shifted the selection onto the post
// below the one selected.
func TestViewport_RemovingAPostAboveKeepsTheSelection(t *testing.T) {
	m := three(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if got := m.SelectedPost().ID; got != "b" {
		t.Fatalf("setup: selected %q, want b", got)
	}

	m.RemovePost("a")

	if got := m.SelectedPost().ID; got != "b" {
		t.Errorf("selected %q after removing a post above, want b still", got)
	}
}

func TestViewport_RemovingTheSelectedPostSelectsANeighbour(t *testing.T) {
	m := three(t)
	m.RemovePost("c")
	if p := m.SelectedPost(); p == nil || p.ID != "b" {
		t.Errorf("selected %v, want b", p)
	}
	if strings.Contains(plain(m), "third") {
		t.Errorf("removed post still shown:\n%s", plain(m))
	}
}

func TestViewport_SetPostsTagsShowsThem(t *testing.T) {
	m := three(t)
	m.SetPostsTags(map[string][]string{"a": {"urgent"}, "c": {"ops"}})
	view := plain(m)
	if !strings.Contains(view, "#urgent") || !strings.Contains(view, "#ops") {
		t.Errorf("tags not shown:\n%s", view)
	}
}

func TestViewport_Loading(t *testing.T) {
	m := viewport.New(testutil.Styles())
	m.SetSize(80, 20)
	m.SetLoading(true)
	if !m.Loading() || !strings.Contains(plain(m), "Loading") {
		t.Errorf("Loading() = %v; view:\n%s", m.Loading(), plain(m))
	}
	m.SetLoading(false)
	if m.Loading() {
		t.Error("still loading after SetLoading(false)")
	}
}
