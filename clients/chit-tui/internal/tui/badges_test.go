package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
)

// twoChannels is a session in team t1 with c1 open and c2 holding ten
// messages.
func twoChannels(t *testing.T, client *mockClient) (tui.Model, []tea.Msg) {
	t.Helper()
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, cmd := step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", TeamID: "t1", DisplayName: "General", TotalMsgCount: 5},
		{ID: "c2", TeamID: "t1", DisplayName: "Random", TotalMsgCount: 10},
	}})
	return m, messagesOf(cmd)
}

// Team channels had no badges until opened: only DMs and the open channel
// had member rows, which the counts come from. They now load for the whole
// team at once.
func TestModel_TeamChannelBadgesShowAtStartup(t *testing.T) {
	client := &mockClient{myMembers: []*model.ChannelMember{
		{ChannelID: "c1", UserID: "u1", MsgCount: 0},
		{ChannelID: "c2", UserID: "u1", MsgCount: 4, MentionCount: 2},
	}}
	m, msgs := twoChannels(t, client)
	for _, msg := range msgs {
		m, _ = step(t, m, msg)
	}

	if got := m.UnreadCount("c2"); got != 6 {
		t.Errorf("unread in c2 = %d, want 10 - 4", got)
	}
	if got := m.MentionCount("c2"); got != 2 {
		t.Errorf("mentions in c2 = %d, want 2", got)
	}
	// c1 is open, so whatever its row says, it has been seen.
	if got := m.UnreadCount("c1"); got != 0 {
		t.Errorf("unread in the open channel = %d, want 0", got)
	}
}

// Opening a channel marks it viewed and fetches its members at the same
// time. A member row read before the view landed restored the old count.
func TestModel_OpenChannelBadgeDoesNotComeBack(t *testing.T) {
	m, _ := twoChannels(t, &mockClient{})

	m, _ = step(t, m, tui.ChannelMembersLoadedMsg{ChannelID: "c1", Members: []*model.ChannelMember{
		{ChannelID: "c1", UserID: "u1", MsgCount: 0, MentionCount: 3},
	}})

	if got := m.UnreadCount("c1"); got != 0 {
		t.Errorf("unread in the open channel = %d, want 0", got)
	}
	if got := m.MentionCount("c1"); got != 0 {
		t.Errorf("mentions in the open channel = %d, want 0", got)
	}
}

// Your own message, sent from another device, is not unread.
func TestModel_OwnPostFromElsewhereIsNotUnread(t *testing.T) {
	m, _ := twoChannels(t, &mockClient{})

	m, _ = step(t, m, wsPosted(map[string]any{
		"id": "x1", "channel_id": "c2", "user_id": "u1", "message": "from my phone", "create_at": 1700000000000,
	}))

	if got := m.UnreadCount("c2"); got != 0 {
		t.Errorf("unread = %d after your own post, want 0", got)
	}
}

// A mention in the channel being read is seen as it arrives.
func TestModel_MentionInTheOpenChannelIsNotCounted(t *testing.T) {
	m, _ := twoChannels(t, &mockClient{})

	m, _ = step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventMentioned,
		Data:  map[string]any{"channel_id": "c1", "post_id": "x1", "user_id": "u2"},
		// As the server sends it: the channel in the data, only the
		// recipient in the broadcast.
		Broadcast: &model.WebSocketBroadcast{UserID: "u1"},
	}})

	if got := m.MentionCount("c1"); got != 0 {
		t.Errorf("mentions in the open channel = %d, want 0", got)
	}
}

// DM member rows can arrive before the signed-in user has loaded. Their
// counts were skipped then and never worked out again.
func TestModel_DMBadgesAfterTheUserLoadsLate(t *testing.T) {
	cfgClient := &mockClient{}
	m := tui.NewModel(nil, cfgClient, nil, testutil.Styles(), nil, nil, nil)
	m, _ = step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{
		{ID: "d1", Type: model.ChannelDirect, Name: "u1__u2", TotalMsgCount: 3},
	}})
	m, _ = step(t, m, tui.ChannelMembersLoadedMsg{ChannelID: "d1", Members: []*model.ChannelMember{
		{ChannelID: "d1", UserID: "u1", MsgCount: 1},
	}})

	m, _ = step(t, m, tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})

	if got := m.UnreadCount("d1"); got != 2 {
		t.Errorf("unread in the DM = %d, want 3 - 1", got)
	}
}
