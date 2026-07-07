package thread_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/thread"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestThread_SetThread(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	root := &model.Post{ID: "root", UserID: "u1", Content: "Root post", CreateAt: 1700000000000}
	reply := &model.Post{ID: "r1", UserID: "u2", Content: "Reply", RootID: "root", CreateAt: 1700000001000}
	m.SetThread(root, []*model.Post{reply})

	if !m.Visible() {
		t.Error("expected visible after SetThread")
	}
	if m.RootPost().ID != "root" {
		t.Errorf("root post ID = %q", m.RootPost().ID)
	}
	if len(m.Replies()) != 1 {
		t.Errorf("replies = %d, want 1", len(m.Replies()))
	}
}

func TestThread_View(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "Root post", CreateAt: 1700000000000},
		[]*model.Post{{ID: "r1", UserID: "u2", Content: "Reply", RootID: "root", CreateAt: 1700000001000}},
	)
	m.SetUsernames(map[string]string{"u1": "alice", "u2": "bob"})

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Root post") {
		t.Errorf("expected root post in view:\n%s", view)
	}
	if !strings.Contains(view, "Reply") {
		t.Errorf("expected reply in view:\n%s", view)
	}
}

func TestThread_ReplyMsg(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
		nil,
	)
	m.Focus()

	// Type a reply
	for _, ch := range "my reply" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	reply, ok := msg.(thread.ReplyMsg)
	if !ok {
		t.Fatalf("expected ReplyMsg, got %T", msg)
	}
	if reply.RootID != "root" {
		t.Errorf("root ID = %q", reply.RootID)
	}
	if reply.Content != "my reply" {
		t.Errorf("content = %q", reply.Content)
	}
}

func TestThread_Toggle(t *testing.T) {
	m := thread.New(testStyles())
	if m.Visible() {
		t.Error("should not be visible initially")
	}
	m.Toggle()
	if !m.Visible() {
		t.Error("should be visible after toggle")
	}
	m.Toggle()
	if m.Visible() {
		t.Error("should not be visible after double toggle")
	}
}

func TestThread_HiddenViewEmpty(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	if m.View() != "" {
		t.Error("expected empty view when hidden")
	}
}

func TestThread_AppendReply(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
		nil,
	)
	m.AppendReply(&model.Post{ID: "r1", UserID: "u2", Content: "Reply", CreateAt: 1700000001000})

	if len(m.Replies()) != 1 {
		t.Errorf("replies = %d, want 1", len(m.Replies()))
	}
}

func TestThread_SetCurrentUsername(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "hey @alice", CreateAt: 1700000000000},
		nil,
	)
	m.SetCurrentUsername("alice")

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "@alice") {
		t.Errorf("expected '@alice' in view after SetCurrentUsername:\n%s", view)
	}
}

func TestThread_SetUsernames(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
		[]*model.Post{{ID: "r1", UserID: "u2", Content: "Reply", RootID: "root", CreateAt: 1700000001000}},
	)

	m.SetUsernames(map[string]string{"u1": "alice", "u2": "bob"})
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "alice") {
		t.Errorf("expected 'alice' in view:\n%s", view)
	}
	if !strings.Contains(view, "bob") {
		t.Errorf("expected 'bob' in view:\n%s", view)
	}
}

func TestThread_FocusBlur(t *testing.T) {
	m := thread.New(testStyles())
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
