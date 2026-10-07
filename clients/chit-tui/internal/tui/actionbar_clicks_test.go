package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// clickBar clicks the action-bar button labeled label, finding it on the
// bar as drawn rather than at a hard-coded column.
func clickBar(t *testing.T, m tui.Model, label string) (tui.Model, tea.Cmd) {
	t.Helper()
	lines := strings.Split(viewOf(m), "\n")
	bar := []rune(lines[len(lines)-1])
	x := strings.Index(string(bar), "["+label)
	if x < 0 {
		t.Fatalf("no %q button on the bar: %q", label, string(bar))
	}
	col := len([]rune(string(bar)[:x])) + 1
	return step(t, m, tea.MouseMsg{X: col, Y: len(lines) - 1, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
}

// Each bar button does what its key does.
func TestModel_ActionBarButtons(t *testing.T) {
	t.Run("search opens the palette in search mode", func(t *testing.T) {
		m, _ := clickBar(t, setupModel(t), "^S Search")
		if m.Focused() != tui.FocusPalette || !strings.Contains(viewOf(m), "?") {
			t.Errorf("focus = %v:\n%s", m.Focused(), viewOf(m))
		}
	})
	t.Run("DM opens the palette for people", func(t *testing.T) {
		m, _ := clickBar(t, setupModel(t), "^D DM")
		if m.Focused() != tui.FocusPalette {
			t.Errorf("focus = %v", m.Focused())
		}
	})
	t.Run("new opens the channel creator", func(t *testing.T) {
		m, _ := clickBar(t, setupModel(t), "^N New")
		if m.Focused() != tui.FocusChCreator {
			t.Errorf("focus = %v", m.Focused())
		}
	})
	t.Run("help opens help", func(t *testing.T) {
		m, _ := clickBar(t, setupModel(t), "? Help")
		if !strings.Contains(viewOf(m), "quit") {
			t.Errorf("help not shown:\n%s", viewOf(m))
		}
	})
	t.Run("back leaves the thread", func(t *testing.T) {
		m := openThread(t, setupModel(t))
		m, _ = clickBar(t, m, "esc Back")
		if strings.Contains(viewOf(m), "esc Back") {
			t.Errorf("still in the thread:\n%s", viewOf(m))
		}
	})
	t.Run("reply opens the selected post's thread", func(t *testing.T) {
		m, _ := step(t, setupModel(t), tea.KeyMsg{Type: tea.KeyTab})
		_, cmd := clickBar(t, m, "↵ Reply")
		if got := wantMsg[viewport.PostSelectedMsg](t, cmd); got.Post.ID != "p1" {
			t.Errorf("opened %q, want p1", got.Post.ID)
		}
	})
}

// /threads opens the inbox and loads the followed threads.
func TestModel_ThreadsCommandLoadsTheInbox(t *testing.T) {
	m := setupModel(t)
	_, cmd := step(t, m, input.SlashTriggerMsg{Input: "/threads"})
	wantMsg[tui.ThreadsLoadedMsg](t, cmd)
}

// A single post's tags, as fetched after tagging it, show on the post.
func TestModel_PostTagsLoadedShowsThem(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.PostTagsLoadedMsg{PostID: "p1", Tags: []*model.Tag{{ID: "t1", Name: "deploy"}}})
	if !strings.Contains(viewOf(m), "#deploy") {
		t.Errorf("tag not shown:\n%s", viewOf(m))
	}
}
