package thread_test

import (
	"strings"
	"testing"

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

func TestThread_Clear(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(
		&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
		[]*model.Post{{ID: "r1", UserID: "u2", Content: "Reply", RootID: "root", CreateAt: 1700000001000}},
	)

	m.Clear()
	if m.RootPost() != nil {
		t.Error("expected no root post after Clear")
	}
	if len(m.Replies()) != 0 {
		t.Error("expected no replies after Clear")
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

// The HTTP response and the WebSocket echo both carry your own reply, so it
// can be appended twice.
func TestThread_AppendReplyIgnoresARepeat(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000}, nil)
	reply := &model.Post{ID: "r1", UserID: "u1", Content: "Reply", RootID: "root", CreateAt: 1700000001000}

	m.AppendReply(reply)
	m.AppendReply(reply)

	if n := len(m.Replies()); n != 1 {
		t.Errorf("replies = %d, want 1", n)
	}
}

func TestThread_RemoveReply(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(40, 20)
	m.SetThread(&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
		[]*model.Post{{ID: "r1", UserID: "u1", Content: "doomed reply", RootID: "root", CreateAt: 1700000001000}})

	if !m.RemoveReply("r1") {
		t.Fatal("RemoveReply reported the reply missing")
	}
	if view := testutil.StripANSI(m.View()); strings.Contains(view, "doomed reply") {
		t.Errorf("removed reply still shown:\n%s", view)
	}
	if m.RemoveReply("r1") {
		t.Error("RemoveReply found a reply already removed")
	}
}

// Tags set on an open thread did not show until something else redrew it.
func TestThread_SetPostTagsRedraws(t *testing.T) {
	m := thread.New(testStyles())
	m.SetSize(60, 20)
	m.SetThread(&model.Post{ID: "root", UserID: "u1", Content: "Root", CreateAt: 1700000000000}, nil)
	_ = m.View()

	m.SetPostTags(map[string][]*model.Tag{"root": {{ID: "t1", Name: "urgent"}}})

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "urgent") {
		t.Errorf("tag not shown:\n%s", view)
	}
}
