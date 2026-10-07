package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
)

func TestModel_TeamsLoadedFetchesChannelsForAllTeams(t *testing.T) {
	m := testModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	_, cmd := m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{
		{ID: "t1", DisplayName: "Engineering"},
		{ID: "t2", DisplayName: "Design"},
	}})

	requested := map[string]bool{}
	for _, msg := range messagesOf(cmd) {
		if cl, ok := msg.(tui.ChannelsLoadedMsg); ok {
			requested[cl.TeamID] = true
		}
	}
	if !requested["t1"] || !requested["t2"] {
		t.Errorf("expected channel fetches for both teams, got %v", requested)
	}
}

func TestModel_SelectChannelFromOtherTeamSwitchesActiveTeam(t *testing.T) {
	m := setupModel(t)

	// Load a second team's channels.
	updated, _ := m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{
		{ID: "t1", DisplayName: "Engineering"},
		{ID: "t2", DisplayName: "Design"},
	}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t2", Channels: []*model.Channel{
		{ID: "c2", DisplayName: "Mockups", TeamID: "t2"},
	}})
	m = updated.(tui.Model)

	// Selecting the team-2 channel should switch the active team context.
	updated, _ = m.Update(palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", DisplayName: "Mockups", TeamID: "t2"}})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Design > Mockups") {
		t.Errorf("expected action bar to show 'Design > Mockups':\n%s", view)
	}
}

func TestModel_SelectDMChannelKeepsActiveTeam(t *testing.T) {
	m := setupModel(t)

	dmCh := &model.Channel{ID: "dm1", Name: "u1__u2", Type: model.ChannelDirect}
	updated, _ := m.Update(palette.ChannelChosenMsg{Channel: dmCh})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Engineering") {
		t.Errorf("expected active team to survive DM selection:\n%s", view)
	}
}

func TestModel_WSPostIncrementsRootUnread(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event:    model.WebSocketEventPosted,
		Sequence: 1,
		Data:     map[string]any{"id": "p9", "channel_id": "other", "user_id": "u2", "content": "hi"},
	}})
	m = updated.(tui.Model)

	if got := m.UnreadCount("other"); got != 1 {
		t.Errorf("expected root unread count 1 for inactive channel, got %d", got)
	}
}

func TestModel_ChannelViewedResetsRootCounts(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event:    model.WebSocketEventPosted,
		Sequence: 1,
		Data:     map[string]any{"id": "p9", "channel_id": "other", "user_id": "u2", "content": "hi"},
	}})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.ChannelViewedMsg{ChannelID: "other"})
	m = updated.(tui.Model)

	if got := m.UnreadCount("other"); got != 0 {
		t.Errorf("expected unread reset to 0 after ChannelViewedMsg, got %d", got)
	}
	if got := m.MentionCount("other"); got != 0 {
		t.Errorf("expected mentions reset to 0 after ChannelViewedMsg, got %d", got)
	}
}
