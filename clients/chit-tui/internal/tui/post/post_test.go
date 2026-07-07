package post_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/markdown"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func testRenderer() *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(markdown.StyleConfig(theme.TokyoNight())),
		glamour.WithWordWrap(76),
	)
	return r
}

func TestPostView_ContainsUsername(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "alice") {
		t.Errorf("expected view to contain username 'alice', got:\n%s", view)
	}
}

func TestPostView_ContainsTimestamp(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	// Should contain a time in HH:MM format
	if !strings.Contains(view, ":") {
		t.Errorf("expected view to contain timestamp, got:\n%s", view)
	}
}

func TestPostView_ContainsContent(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello world", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "Hello world") {
		t.Errorf("expected view to contain 'Hello world', got:\n%s", view)
	}
}

func TestPostView_PinnedBadge(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", IsPinned: true, CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "pinned") {
		t.Errorf("expected view to contain pinned badge, got:\n%s", view)
	}
}

func TestPostView_NilPost(t *testing.T) {
	m := post.New(nil, "alice", testStyles(), 80, nil, "", 0, nil)
	if m.View() != "" {
		t.Error("expected empty view for nil post")
	}
}

func TestPostView_Height(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	if m.Height() < 2 {
		t.Errorf("expected height >= 2, got %d", m.Height())
	}
}

func TestPostView_WithRenderer(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, testRenderer(), "", 0, nil)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Hello") {
		t.Errorf("expected content with renderer, got:\n%s", view)
	}
}

func TestPostView_MentionHighlight(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "hello @alice", CreateAt: 1700000000000}
	m := post.New(p, "bob", testStyles(), 80, nil, "testuser", 0, nil)
	view := m.View()
	if !strings.Contains(view, "@alice") {
		t.Errorf("expected '@alice' in view:\n%s", view)
	}
}

func TestPostView_SelfMentionHighlight(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "hey @testuser", CreateAt: 1700000000000}
	m := post.New(p, "bob", testStyles(), 80, nil, "testuser", 0, nil)
	view := m.View()
	if !strings.Contains(view, "@testuser") {
		t.Errorf("expected '@testuser' in view:\n%s", view)
	}
}

func TestPostView_MultipleMentions(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "@alice @bob", CreateAt: 1700000000000}
	m := post.New(p, "charlie", testStyles(), 80, nil, "", 0, nil)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "@alice") || !strings.Contains(view, "@bob") {
		t.Errorf("expected both mentions in view:\n%s", view)
	}
}

func TestPostView_AllMention(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "@all please review", CreateAt: 1700000000000}
	m := post.New(p, "bob", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "@all") {
		t.Errorf("expected '@all' in view:\n%s", view)
	}
}

func TestPostView_ReplyCountBadge(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 3, nil)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "[3 replies]") {
		t.Errorf("expected '[3 replies]' badge in view:\n%s", view)
	}
}

func TestPostView_ZeroReplyCount(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "replies") {
		t.Errorf("expected no reply badge for zero count:\n%s", view)
	}
}

func TestPostView_ReplyIndent(t *testing.T) {
	p := &model.Post{ID: "r1", RootID: "p1", Content: "This is a reply", CreateAt: 1700000000000}
	m := post.New(p, "bob", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "This is a reply") {
		t.Errorf("expected reply content in view:\n%s", view)
	}
	// Reply should have a left border (thick border character)
	if !strings.Contains(view, "┃") {
		t.Errorf("expected left border indicator for reply post:\n%s", view)
	}
}

func TestPostView_NonReplyNoIndent(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Root post", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if strings.Contains(view, "┃") {
		t.Errorf("expected no left border for non-reply post:\n%s", view)
	}
}

func TestPostView_NoMentions_Unchanged(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "no mentions here", CreateAt: 1700000000000}
	m := post.New(p, "bob", testStyles(), 80, nil, "", 0, nil)
	view := m.View()
	if !strings.Contains(view, "no mentions here") {
		t.Errorf("expected unchanged content:\n%s", view)
	}
}

func TestPostView_TagBadges(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, []string{"urgent", "bug"})
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "#urgent") {
		t.Errorf("expected '#urgent' tag badge in view:\n%s", view)
	}
	if !strings.Contains(view, "#bug") {
		t.Errorf("expected '#bug' tag badge in view:\n%s", view)
	}
}

func TestPostView_NoTags(t *testing.T) {
	p := &model.Post{ID: "p1", Content: "Hello", CreateAt: 1700000000000}
	m := post.New(p, "alice", testStyles(), 80, nil, "", 0, nil)
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "#") {
		t.Errorf("expected no tag badges:\n%s", view)
	}
}
