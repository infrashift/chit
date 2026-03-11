package styles_test

import (
	"testing"

	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

func TestNew_ReturnsValidStyles(t *testing.T) {
	s := styles.New(theme.TokyoNight())

	// Verify styles are callable (render empty string without panic).
	_ = s.Sidebar.Render("")
	_ = s.SidebarItem.Render("")
	_ = s.SidebarActive.Render("")
	_ = s.Viewport.Render("")
	_ = s.Input.Render("")
	_ = s.ThreadPanel.Render("")
	_ = s.CmdPalette.Render("")
	_ = s.StatusBar.Render("")
	_ = s.Username.Render("user")
	_ = s.Timestamp.Render("12:00")
	_ = s.UnreadBadge.Render("3")
	_ = s.PinBadge.Render("pinned")
	_ = s.ErrorText.Render("err")
	_ = s.Border.Render("")
	_ = s.ActiveBorder.Render("")
	_ = s.MentionBadge.Render("@2")
	_ = s.MentionText.Render("@alice")
	_ = s.MentionSelfHighlight.Render("@me")
	_ = s.AutocompletePanel.Render("")
	_ = s.AutocompleteItem.Render("item")
	_ = s.AutocompleteActive.Render("active")
	_ = s.SelectedPost.Render("post")
	_ = s.ReplyIndent.Render("reply")

	if s.MarkdownStyleConfig.Document.Color == nil {
		t.Error("expected MarkdownStyleConfig.Document.Color to be set")
	}
}
