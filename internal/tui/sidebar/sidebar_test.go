package sidebar_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit-tui/internal/model"
	"github.com/infrashift/chit-tui/internal/tui/sidebar"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

func testStyles() styles.Styles {
	return styles.New(theme.TokyoNight())
}

func TestSidebar_SetTeams(t *testing.T) {
	m := sidebar.New(testStyles())
	teams := []*model.Team{{ID: "t1", DisplayName: "Engineering"}, {ID: "t2", DisplayName: "Design"}}
	m.SetTeams(teams)

	if m.ActiveTeamID != "t1" {
		t.Errorf("ActiveTeamID = %q, want %q", m.ActiveTeamID, "t1")
	}
	if len(m.Teams) != 2 {
		t.Errorf("Teams count = %d, want 2", len(m.Teams))
	}
}

func TestSidebar_SetChannels(t *testing.T) {
	m := sidebar.New(testStyles())
	channels := []*model.Channel{{ID: "c1", DisplayName: "General"}, {ID: "c2", DisplayName: "Random"}}
	m.SetChannels(channels)

	if m.ActiveChanID != "c1" {
		t.Errorf("ActiveChanID = %q, want %q", m.ActiveChanID, "c1")
	}
	if ch := m.SelectedChannel(); ch.ID != "c1" {
		t.Errorf("SelectedChannel = %q, want %q", ch.ID, "c1")
	}
}

func TestSidebar_NavigateDown(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
		{ID: "c2", DisplayName: "Random"},
	})
	m.Focus()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if ch := m.SelectedChannel(); ch == nil || ch.ID != "c2" {
		t.Errorf("expected cursor on c2, got %+v", ch)
	}
}

func TestSidebar_SelectChannel(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.Focus()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	selected, ok := msg.(sidebar.ChannelSelectedMsg)
	if !ok {
		t.Fatalf("expected ChannelSelectedMsg, got %T", msg)
	}
	if selected.Channel.ID != "c1" {
		t.Errorf("selected channel = %q, want c1", selected.Channel.ID)
	}
}

func TestSidebar_TeamSelect(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetTeams([]*model.Team{{ID: "t1", DisplayName: "Eng"}})
	// Navigate back to teams
	m.SetChannels([]*model.Channel{{ID: "c1"}})
	m.Focus()

	// Press escape to go back to teams
	var escCmd tea.Cmd
	m, escCmd = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if escCmd == nil {
		t.Fatal("expected BackToTeamsMsg command from Esc")
	}
	escMsg := escCmd()
	if _, ok := escMsg.(sidebar.BackToTeamsMsg); !ok {
		t.Fatalf("expected BackToTeamsMsg, got %T", escMsg)
	}

	// Select team
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter on team")
	}
	msg := cmd()
	_, ok := msg.(sidebar.TeamSelectedMsg)
	if !ok {
		t.Fatalf("expected TeamSelectedMsg, got %T", msg)
	}
}

func TestSidebar_EscEmitsBackToTeamsMsg(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.Focus()

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command from Esc in channel view")
	}
	msg := cmd()
	if _, ok := msg.(sidebar.BackToTeamsMsg); !ok {
		t.Fatalf("expected BackToTeamsMsg, got %T", msg)
	}
}

func TestSidebar_EscNoOpInTeamView(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetTeams([]*model.Team{{ID: "t1", DisplayName: "Eng"}})
	m.Focus()

	// Already in team view, Esc should not produce a command
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Error("Esc in team view should not produce a command")
	}
}

func TestSidebar_UnreadBadge(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetUnread("c1", 5)

	view := m.View()
	if !strings.Contains(view, "(5)") {
		t.Errorf("expected unread badge (5) in view:\n%s", view)
	}
}

func TestSidebar_ViewContainsTeams(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetTeams([]*model.Team{{ID: "t1", DisplayName: "Engineering"}})
	// Don't set channels, should show teams

	view := m.View()
	if !strings.Contains(view, "Engineering") {
		t.Errorf("expected 'Engineering' in view:\n%s", view)
	}
}

func TestSidebar_FocusBlur(t *testing.T) {
	m := sidebar.New(testStyles())
	if m.Focused() {
		t.Error("should not be focused initially")
	}
	m.Focus()
	if !m.Focused() {
		t.Error("should be focused after Focus()")
	}
	m.Blur()
	if m.Focused() {
		t.Error("should not be focused after Blur()")
	}
}

func TestSidebar_MentionBadge(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetMention("c1", 3)

	view := m.View()
	if !strings.Contains(view, "[@3]") {
		t.Errorf("expected mention badge [@3] in view:\n%s", view)
	}
}

func TestSidebar_MentionAndUnreadBadges(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetUnread("c1", 5)
	m.SetMention("c1", 2)

	view := m.View()
	if !strings.Contains(view, "(5)") {
		t.Errorf("expected unread badge (5) in view:\n%s", view)
	}
	if !strings.Contains(view, "[@2]") {
		t.Errorf("expected mention badge [@2] in view:\n%s", view)
	}
}

func TestSidebar_ZeroMentionNoBadge(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetMention("c1", 0)

	view := m.View()
	if strings.Contains(view, "[@") {
		t.Errorf("expected no mention badge for zero count:\n%s", view)
	}
}

func TestSidebar_IgnoresInputWhenBlurred(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(30, 20)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
		{ID: "c2", DisplayName: "Random"},
	})
	// Do NOT focus

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if ch := m.SelectedChannel(); ch == nil || ch.ID != "c1" {
		t.Errorf("cursor should not have moved: %+v", ch)
	}
}

func TestSidebar_DMSection(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m.SetDMDisplayName("dm1", "alice")

	view := m.View()
	if !strings.Contains(view, "Direct Messages") {
		t.Errorf("expected 'Direct Messages' header in view:\n%s", view)
	}
	if !strings.Contains(view, "alice") {
		t.Errorf("expected 'alice' in DM section:\n%s", view)
	}
}

func TestSidebar_NavigateToDM(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m.SetDMDisplayName("dm1", "alice")
	m.Focus()

	// Down past channels into DM section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	ch := m.SelectedChannel()
	if ch == nil || ch.ID != "dm1" {
		t.Errorf("expected DM channel dm1, got %+v", ch)
	}
}

func TestSidebar_SelectDMChannel(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m.Focus()

	// Navigate down to DM
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Select
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter on DM")
	}
	msg := cmd()
	selected, ok := msg.(sidebar.ChannelSelectedMsg)
	if !ok {
		t.Fatalf("expected ChannelSelectedMsg, got %T", msg)
	}
	if selected.Channel.ID != "dm1" {
		t.Errorf("selected channel = %q, want dm1", selected.Channel.ID)
	}
}

func TestSidebar_DMDisplayNameFallback(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", DisplayName: "u1, u2", Type: "D"},
	})
	// Don't set display name — should fall back to DisplayName

	view := m.View()
	if !strings.Contains(view, "u1, u2") {
		t.Errorf("expected fallback display name 'u1, u2' in view:\n%s", view)
	}
}

func TestSidebar_DMUnreadBadge(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m.SetDMDisplayName("dm1", "alice")
	m.SetUnread("dm1", 3)

	view := m.View()
	if !strings.Contains(view, "(3)") {
		t.Errorf("expected unread badge (3) on DM:\n%s", view)
	}
}

func TestSidebar_NoDMSectionWhenEmpty(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	// No DM channels

	view := m.View()
	if strings.Contains(view, "Direct Messages") {
		t.Errorf("should not show Direct Messages header when no DMs:\n%s", view)
	}
}

func TestSidebar_CrossSectionNavBounds(t *testing.T) {
	m := sidebar.New(testStyles())
	m.SetSize(40, 30)
	m.SetChannels([]*model.Channel{
		{ID: "c1", DisplayName: "General"},
	})
	m.SetDMChannels([]*model.Channel{
		{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m.Focus()

	// Navigate down twice (past DM end)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Should clamp to last item (DM)
	ch := m.SelectedChannel()
	if ch == nil || ch.ID != "dm1" {
		t.Errorf("expected dm1 at bounds, got %+v", ch)
	}

	// Navigate up twice (back to c1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	ch = m.SelectedChannel()
	if ch == nil || ch.ID != "c1" {
		t.Errorf("expected c1 after up, got %+v", ch)
	}
}
