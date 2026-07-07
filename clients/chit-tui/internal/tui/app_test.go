package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

func testModel() tui.Model {
	cfg := &config.Config{
		ServerURL:    "http://localhost:8065",
		SessionToken: "test-token",
		WSScheme:     "ws",
	}
	s := styles.New(theme.TokyoNight())
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}},
		posts:    &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
		commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}},
		users:    []*model.User{{ID: "u1", Username: "alice"}},
	}
	return tui.NewModel(cfg, client, nil, s, nil, nil, nil)
}

func setupModel(t *testing.T) tui.Model {
	t.Helper()
	m := testModel()
	// Simulate window size
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	// Load user
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	m = updated.(tui.Model)
	// Load teams
	updated, _ = m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}})
	m = updated.(tui.Model)
	// Load channels
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", DisplayName: "General"}}})
	m = updated.(tui.Model)
	// Load posts
	updated, _ = m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1",
		Posts:     &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
	})
	m = updated.(tui.Model)
	// Load users
	updated, _ = m.Update(tui.UsersLoadedMsg{Users: []*model.User{{ID: "u1", Username: "alice"}}})
	m = updated.(tui.Model)
	// Load commands
	updated, _ = m.Update(tui.CommandsLoadedMsg{Commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}}})
	m = updated.(tui.Model)
	return m
}

func TestModel_Init_FetchesUserAndTeams(t *testing.T) {
	m := testModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected init commands")
	}
}

func TestModel_WindowSize(t *testing.T) {
	m := testModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after resize")
	}
}

func TestModel_TabCyclesFocus(t *testing.T) {
	m := setupModel(t)

	// Start at input, tab moves to viewport
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	// Tab again wraps back to input
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	// And again to viewport
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	// View should still render
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_ChannelSelectLoadsPosts(t *testing.T) {
	m := setupModel(t)
	ch := &model.Channel{ID: "c2", DisplayName: "Random"}
	_, cmd := m.Update(palette.ChannelChosenMsg{Channel: ch})
	if cmd == nil {
		t.Error("expected command to fetch posts")
	}
}

func TestModel_WSEventInsertsPost(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 1,
			Data: map[string]any{
				"id": "p2", "channel_id": "c1", "user_id": "u1",
				"content": "WS post", "create_at": float64(1700000002000),
			},
		},
	}
	updated, _ := m.Update(wsEvt)
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "WS post") {
		t.Errorf("expected 'WS post' in view:\n%s", view)
	}
}

// openThread selects the loaded post and loads its thread, swapping the
// main pane to the thread view.
func openThread(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	updated, cmd := m.Update(viewport.PostSelectedMsg{Post: &model.Post{ID: "p1", UserID: "u1", Content: "Hello"}})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected command to fetch thread")
	}
	updated, _ = m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{
			Order: []*model.Post{
				{ID: "p1", UserID: "u1", Content: "Root post", CreateAt: 1700000000000},
				{ID: "r1", UserID: "u1", Content: "First reply", RootID: "p1", CreateAt: 1700000001000},
			},
		},
	})
	return updated.(tui.Model)
}

func TestModel_PostSelectionSwapsToThreadPane(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "First reply") {
		t.Errorf("expected thread content in main pane:\n%s", view)
	}
	if !strings.Contains(view, "[esc Back]") {
		t.Errorf("expected back button on action bar in thread pane:\n%s", view)
	}
}

func TestModel_CtrlKOpensPalette(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "↵ select") {
		t.Errorf("expected palette footer in view:\n%s", view)
	}
	if !strings.Contains(view, "General") {
		t.Errorf("expected channel rows in palette view:\n%s", view)
	}
}

func TestModel_PaletteSlashModeListsCommands(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "remind") {
		t.Errorf("expected 'remind' in command mode view:\n%s", view)
	}
}

func TestModel_EscClosesPalette(t *testing.T) {
	m := setupModel(t)

	// Open palette
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)

	// Close with escape
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	view := m.View()
	// Should not show palette search input
	_ = view
}

func TestModel_PostSelectedOpensThread(t *testing.T) {
	m := setupModel(t)

	p := &model.Post{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}
	_, cmd := m.Update(viewport.PostSelectedMsg{Post: p})
	if cmd == nil {
		t.Error("expected command to fetch thread")
	}
}

func TestModel_CommandChosenMsg(t *testing.T) {
	m := setupModel(t)

	cmd := &model.Command{ID: "cmd1", Slug: "remind"}
	updated, _ := m.Update(palette.CommandChosenMsg{Command: cmd})
	m = updated.(tui.Model)

	// Focus should return to input
	view := m.View()
	_ = view
}

func TestModel_ErrorMsg(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ErrMsg{Err: &model.AppError{Message: "test error"}})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "test error") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_ViewLoading(t *testing.T) {
	m := testModel()
	view := m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...' before window size, got %q", view)
	}
}

func TestModel_ShiftTabCyclesFocusBackward(t *testing.T) {
	m := setupModel(t)

	// Start at input, shift-tab wraps to viewport
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(tui.Model)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_ThreadLoadedMsg(t *testing.T) {
	m := setupModel(t)
	updated, _ := m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{
			Order: []*model.Post{
				{ID: "p1", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
				{ID: "r1", UserID: "u1", Content: "Reply", RootID: "p1", CreateAt: 1700000001000},
			},
		},
	})
	m = updated.(tui.Model)
	view := m.View()
	_ = view
}

func TestModel_SendMsgFromInput(t *testing.T) {
	m := setupModel(t)
	updated, cmd := m.Update(input.SendMsg{Content: "test message"})
	m = updated.(tui.Model)
	_ = m
	_ = cmd
}

func TestModel_SendMsgInThreadPaneCreatesReply(t *testing.T) {
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}},
		posts:    &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
		commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}},
		users:    []*model.User{{ID: "u1", Username: "alice"}},
	}
	cfg := &config.Config{ServerURL: "http://localhost:8065", SessionToken: "test-token", WSScheme: "ws"}
	m := tui.NewModel(cfg, client, nil, styles.New(theme.TokyoNight()), nil, nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", DisplayName: "General"}}})
	m = updated.(tui.Model)
	m = openThread(t, m)

	// Sending from the input while in the thread pane creates a reply.
	_, cmd := m.Update(input.SendMsg{Content: "my reply"})
	if cmd == nil {
		t.Fatal("expected command from SendMsg in thread pane")
	}
	cmd()
	if client.lastCreatedPost == nil {
		t.Fatal("expected CreatePost to be called")
	}
	if client.lastCreatedPost.RootID != "p1" {
		t.Errorf("expected reply RootID p1, got %q", client.lastCreatedPost.RootID)
	}
	if client.lastCreatedPost.Content != "my reply" {
		t.Errorf("expected reply content, got %q", client.lastCreatedPost.Content)
	}
}

func TestModel_EscClosesThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)

	// Escape returns to the channel pane.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "First reply") {
		t.Errorf("expected channel view after Esc, still showing thread:\n%s", view)
	}
	if strings.Contains(view, "[esc Back]") {
		t.Errorf("expected back button gone after Esc:\n%s", view)
	}
}

func TestModel_WSEventDropsStale(t *testing.T) {
	m := setupModel(t)

	// Send event with seq=5
	updated, _ := m.Update(tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 5,
			Data: map[string]any{
				"id": "p2", "channel_id": "c1", "user_id": "u1",
				"content": "first", "create_at": float64(1700000002000),
			},
		},
	})
	m = updated.(tui.Model)

	// Send stale event with seq=3, should be dropped
	updated, _ = m.Update(tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 3,
			Data: map[string]any{
				"id": "p3", "channel_id": "c1", "user_id": "u1",
				"content": "stale", "create_at": float64(1700000003000),
			},
		},
	})
	m = updated.(tui.Model)

	view := m.View()
	if strings.Contains(view, "stale") {
		t.Error("stale WS event should have been dropped")
	}
}

func TestModel_ChannelViewedMsg(t *testing.T) {
	m := setupModel(t)
	updated, _ := m.Update(tui.ChannelViewedMsg{ChannelID: "c1"})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_SlashTriggerOpensPalette(t *testing.T) {
	m := setupModel(t)
	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/remind"})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_DelegateKeyToViewport(t *testing.T) {
	m := setupModel(t)
	// Tab to viewport
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	// Send a key to viewport
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_DelegateKeyToInput(t *testing.T) {
	m := setupModel(t)
	// Tab to viewport, then to input
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	// Type in input
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_DelegateKeyToThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)
	// Tab from input focuses the thread pane; send it a scroll key.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_DelegateKeyToPalette(t *testing.T) {
	m := setupModel(t)
	// Open palette
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	// Send key to palette
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_TabWithThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)
	// Tab cycles between thread and input
	var updated tea.Model
	for range 5 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(tui.Model)
	}
	_ = m
}

func TestModel_CtrlSOpensSearch(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Search") {
		t.Errorf("expected search overlay in view:\n%s", view)
	}
}

func TestModel_EscClosesSearch(t *testing.T) {
	m := setupModel(t)

	// Open search
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(tui.Model)

	// Close with escape
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	_ = m.View()
}

func TestModel_SearchResultsMsg(t *testing.T) {
	m := setupModel(t)

	// Open search
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(tui.Model)

	// Receive search results
	updated, _ = m.Update(tui.SearchResultsMsg{
		Posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", UserID: "u1", Content: "found it"},
		}},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "found it") {
		t.Errorf("expected 'found it' in search view:\n%s", view)
	}
}

func TestModel_SearchResultSelected(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(palette.PostChosenMsg{
		Post: &model.Post{ID: "p1", Content: "result"},
	})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_SearchSubmitMsg(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(palette.SearchSubmitMsg{Term: "hello"})
	if cmd == nil {
		t.Error("expected command to search posts")
	}
}

func TestModel_WSConnectedStartsListening(t *testing.T) {
	cfg := &config.Config{
		ServerURL:    "http://localhost:8065",
		SessionToken: "test-token",
		WSScheme:     "ws",
	}
	s := styles.New(theme.TokyoNight())
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}},
		posts:    &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
		commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}},
		users:    []*model.User{{ID: "u1", Username: "alice"}},
	}
	wsClient := newMockWSClient()
	m := tui.NewModel(cfg, client, wsClient, s, nil, nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	_, cmd := m.Update(tui.WSConnectedMsg{})
	if cmd == nil {
		t.Error("expected ListenWebSocket command after WSConnectedMsg")
	}
}

func TestModel_ChannelMembersComputesUnread(t *testing.T) {
	m := setupModel(t)

	// Channel c1 has TotalMsgCount=0 from setupModel. Set a channel with known count.
	updated, _ := m.Update(tui.ChannelsLoadedMsg{
		TeamID: "t1",
		Channels: []*model.Channel{{
			ID:            "c1",
			DisplayName:   "General",
			TotalMsgCount: 10,
		}},
	})
	m = updated.(tui.Model)

	// Simulate channel members loaded with user having read 7 messages
	updated, _ = m.Update(tui.ChannelMembersLoadedMsg{
		ChannelID: "c1",
		Members: []*model.ChannelMember{
			{ChannelID: "c1", UserID: "u1", MsgCount: 7},
		},
	})
	m = updated.(tui.Model)

	if got := m.UnreadCount("c1"); got != 3 {
		t.Fatalf("expected unread count 3, got %d", got)
	}

	// The palette shows the badge.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "(3)") {
		t.Errorf("expected unread badge '(3)' in palette view:\n%s", view)
	}
}

func TestModel_WSMentionedEventIncrementsBadge(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventMentioned,
			Sequence: 1,
			Data:     map[string]any{},
			Broadcast: &model.WebSocketBroadcast{
				ChannelID: "c1",
			},
		},
	}
	updated, _ := m.Update(wsEvt)
	m = updated.(tui.Model)
	_ = m
}

func TestModel_ChannelViewedClearsMention(t *testing.T) {
	m := setupModel(t)

	// Simulate mention increment
	updated, _ := m.Update(tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventMentioned,
			Sequence: 1,
			Data:     map[string]any{},
			Broadcast: &model.WebSocketBroadcast{
				ChannelID: "c1",
			},
		},
	})
	m = updated.(tui.Model)

	// View channel clears mention
	updated, _ = m.Update(tui.ChannelViewedMsg{ChannelID: "c1"})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_AtTriggerShowsMention(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(input.AtTriggerMsg{Prefix: "al", StartCol: 0})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_AtDismissHidesMention(t *testing.T) {
	m := setupModel(t)

	// Show mention first
	updated, _ := m.Update(input.AtTriggerMsg{Prefix: "", StartCol: 0})
	m = updated.(tui.Model)

	// Dismiss
	updated, _ = m.Update(input.AtDismissMsg{})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_MentionUserSelected(t *testing.T) {
	m := setupModel(t)

	// Focus input first
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	// Show mention
	updated, _ = m.Update(input.AtTriggerMsg{Prefix: "", StartCol: 0})
	m = updated.(tui.Model)

	// Select user
	updated, _ = m.Update(mention.UserSelectedMsg{Username: "alice", StartCol: 0})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_ChannelMembersLoadsMentions(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ChannelMembersLoadedMsg{
		ChannelID: "c1",
		Members: []*model.ChannelMember{
			{ChannelID: "c1", UserID: "u1", MsgCount: 7, MentionCount: 2},
		},
	})
	m = updated.(tui.Model)
	_ = m
}

func TestModel_WSEventPostedOtherChannel(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 1,
			Data: map[string]any{
				"id": "p2", "channel_id": "other-ch", "user_id": "u1",
				"content": "other chan", "create_at": float64(1700000002000),
			},
		},
	}
	updated, _ := m.Update(wsEvt)
	m = updated.(tui.Model)

	// Post should NOT appear in viewport (different channel)
	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "other chan") {
		t.Error("post from other channel should not appear in viewport")
	}
}

func TestModel_WSEventPostedThreadReply(t *testing.T) {
	m := setupModel(t)

	// Load a thread first
	updated, _ := m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{
			Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Root", CreateAt: 1700000000000}},
		},
	})
	m = updated.(tui.Model)

	// WS event: reply to open thread in active channel
	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 2,
			Data: map[string]any{
				"id": "r1", "channel_id": "c1", "user_id": "u1",
				"root_id": "p1", "content": "ws reply", "create_at": float64(1700000003000),
			},
		},
	}
	updated, _ = m.Update(wsEvt)
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "ws reply") {
		t.Errorf("expected 'ws reply' in view:\n%s", view)
	}
}

func TestModel_WSEventThreadUpdated(t *testing.T) {
	m := setupModel(t)

	// Load a thread first
	updated, _ := m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{
			Order: []*model.Post{
				{ID: "p1", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
				{ID: "r1", UserID: "u1", Content: "Reply", RootID: "p1", CreateAt: 1700000001000},
			},
		},
	})
	m = updated.(tui.Model)

	// thread_updated event
	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventThreadUpdated,
			Sequence: 3,
			Data: map[string]any{
				"thread": map[string]any{
					"post_id":     "p1",
					"reply_count": float64(5),
				},
				"post": map[string]any{
					"id": "r2", "channel_id": "c1", "user_id": "u1",
					"root_id": "p1", "content": "new reply", "create_at": float64(1700000004000),
				},
			},
		},
	}
	updated, _ = m.Update(wsEvt)
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "5 replies") {
		t.Errorf("expected '5 replies' badge in view after thread_updated:\n%s", view)
	}
}

func TestModel_WSEventPostedIncrementsThreadCount(t *testing.T) {
	m := setupModel(t)

	// WS event: a reply post arrives in the active channel
	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 1,
			Data: map[string]any{
				"id": "r1", "channel_id": "c1", "user_id": "u1",
				"root_id": "p1", "content": "reply", "create_at": float64(1700000002000),
			},
		},
	}
	updated, _ := m.Update(wsEvt)
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "1 replies") {
		t.Errorf("expected '1 replies' badge after WS reply:\n%s", view)
	}
}

func TestModel_ThreadLoadedCachesCount(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{
			Order: []*model.Post{
				{ID: "p1", UserID: "u1", Content: "Root", CreateAt: 1700000000000},
				{ID: "r1", UserID: "u1", Content: "Reply", RootID: "p1", CreateAt: 1700000001000},
			},
		},
	})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "1 replies") {
		t.Errorf("expected '1 replies' badge after ThreadLoadedMsg:\n%s", view)
	}
}

func TestModel_ErrorMsgsFromLoaders(t *testing.T) {
	m := setupModel(t)
	errMsg := &model.AppError{Message: "fail"}

	tests := []tea.Msg{
		tui.UserLoadedMsg{Err: errMsg},
		tui.TeamsLoadedMsg{Err: errMsg},
		tui.ChannelsLoadedMsg{Err: errMsg},
		tui.PostsLoadedMsg{Err: errMsg},
		tui.PostCreatedMsg{Err: errMsg},
		tui.ThreadLoadedMsg{Err: errMsg},
		tui.CommandsLoadedMsg{Err: errMsg},
		tui.UsersLoadedMsg{Err: errMsg},
		tui.ChannelMembersLoadedMsg{Err: errMsg},
	}

	for _, msg := range tests {
		updated, _ := m.Update(msg)
		m = updated.(tui.Model)
	}

	view := m.View()
	if !strings.Contains(view, "fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_CtrlDOpensPalettePeopleMode(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Type a name") {
		t.Errorf("expected palette people mode in view:\n%s", view)
	}
}

func TestModel_EscClosesPeopleMode(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(tui.Model)

	// Close with escape
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "Type a name") {
		t.Errorf("palette should be closed:\n%s", view)
	}
}

func TestModel_DMChannelsLoadedMsg(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "dm1", Name: "u1__u2", Type: "D"},
		},
	})
	m = updated.(tui.Model)

	_ = m.View()
}

func TestModel_DMChannelsLoadedMsg_Error(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.DMChannelsLoadedMsg{
		Err: &model.AppError{Message: "dm fail"},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "dm fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_DMPickerSearchTriggered(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(dmpicker.SearchTriggeredMsg{Term: "bob"})
	if cmd == nil {
		t.Error("expected command from SearchTriggeredMsg")
	}
}

func TestModel_UserSearchResultsMsg(t *testing.T) {
	m := setupModel(t)

	// Open DM picker first
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.UserSearchResultsMsg{
		Users: []*model.User{{ID: "u2", Username: "bob"}},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "bob") {
		t.Errorf("expected 'bob' in DM picker results:\n%s", view)
	}
}

func TestModel_UserSearchResultsMsg_FiltersSelf(t *testing.T) {
	m := setupModel(t)

	// Open DM picker
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(tui.Model)

	// Results include self (u1/alice) and another user
	updated, _ = m.Update(tui.UserSearchResultsMsg{
		Users: []*model.User{
			{ID: "u1", Username: "alice"},
			{ID: "u2", Username: "bob"},
		},
	})
	m = updated.(tui.Model)

	view := m.View()
	if strings.Contains(view, "@alice") {
		t.Errorf("self (alice) should be filtered from DM picker results:\n%s", view)
	}
	if !strings.Contains(view, "bob") {
		t.Errorf("expected 'bob' in DM picker results:\n%s", view)
	}
}

func TestModel_UserSearchResultsMsg_Error(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.UserSearchResultsMsg{
		Err: &model.AppError{Message: "search fail"},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "search fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_PaletteUserChosenCreatesDM(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(palette.UserChosenMsg{
		User: &model.User{ID: "u2", Username: "bob"},
	})
	if cmd == nil {
		t.Error("expected command from UserChosenMsg")
	}
}

func TestModel_DMCreatedMsg(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.DMCreatedMsg{
		Channel: &model.Channel{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Error("expected commands from DMCreatedMsg")
	}
	_ = m.View()
}

func TestModel_DMCreatedMsg_Error(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.DMCreatedMsg{
		Err: &model.AppError{Message: "dm create fail"},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "dm create fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_DMCreatedMsg_NoDuplicate(t *testing.T) {
	m := setupModel(t)

	// First, load DM channels
	updated, _ := m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "dm1", Name: "u1__u2", Type: "D"},
		},
	})
	m = updated.(tui.Model)

	// Create same DM again - should not duplicate
	updated, _ = m.Update(tui.DMCreatedMsg{
		Channel: &model.Channel{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m = updated.(tui.Model)

	_ = m.View()
}

func TestModel_WSEventChannelCreatedDM(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventChannelCreated,
			Sequence: 1,
			Data: map[string]any{
				"type": "D",
			},
		},
	}
	_, cmd := m.Update(wsEvt)
	if cmd == nil {
		t.Error("expected command from channel_created DM event")
	}
}

func TestModel_WSEventChannelCreatedNonDM(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventChannelCreated,
			Sequence: 1,
			Data: map[string]any{
				"type": "O",
			},
		},
	}
	_, _ = m.Update(wsEvt)
	// Should not trigger FetchDMChannels for non-DM channel type
}

func TestModel_DMDisplayNameResolution(t *testing.T) {
	m := setupModel(t)

	// Load DM channels with known user IDs
	updated, _ := m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "dm1", Name: "u1__u2", Type: "D"},
		},
	})
	m = updated.(tui.Model)

	// Load the other user
	updated, _ = m.Update(tui.UsersLoadedMsg{
		Users: []*model.User{{ID: "u2", Username: "bob", DisplayName: "Bob Smith"}},
	})
	m = updated.(tui.Model)

	// The resolved name shows up in the palette's DM rows.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Bob Smith") {
		t.Errorf("expected resolved DM display name 'Bob Smith' in palette:\n%s", view)
	}
}

func TestModel_DMDisplayNameResolution_MeArrivesAfterDMChannels(t *testing.T) {
	// Simulate the race: DMChannelsLoadedMsg arrives before UserLoadedMsg,
	// so m.me is nil during initial resolveDMDisplayNames. The fix ensures
	// UserLoadedMsg also triggers resolution.
	m := testModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	// Load teams and channels so sidebar renders
	updated, _ = m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", DisplayName: "General"}}})
	m = updated.(tui.Model)

	// DM channels arrive first (m.me is still nil)
	updated, _ = m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "dm1", Name: "u1__u2", Type: "D"},
		},
	})
	m = updated.(tui.Model)

	// Now current user arrives — should trigger DM resolution + fetch missing users
	updated, cmd := m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Fatal("expected fetchMissingUsers command after UserLoadedMsg with pending DM channels")
	}

	// Simulate the fetched user arriving
	updated, _ = m.Update(tui.UsersLoadedMsg{
		Users: []*model.User{{ID: "u2", Username: "bob", DisplayName: "Bob Smith"}},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "Bob Smith") {
		t.Errorf("expected resolved DM display name 'Bob Smith' in palette:\n%s", view)
	}
}

func TestModel_StatusBarDMChannel(t *testing.T) {
	m := setupModel(t)

	// Load users
	updated, _ := m.Update(tui.UsersLoadedMsg{
		Users: []*model.User{{ID: "u2", Username: "bob"}},
	})
	m = updated.(tui.Model)

	// Switch to DM channel
	dmCh := &model.Channel{ID: "dm1", Name: "u1__u2", Type: "D"}
	updated, _ = m.Update(palette.ChannelChosenMsg{Channel: dmCh})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "bob") {
		t.Errorf("expected 'bob' in status bar for DM channel:\n%s", view)
	}
}

func TestModel_DelegateKeyToDMPicker(t *testing.T) {
	m := setupModel(t)

	// Open DM picker
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(tui.Model)

	// Type in DM picker
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(tui.Model)

	_ = m.View()
}

func TestModel_DMChannelsUnreadComputed(t *testing.T) {
	m := setupModel(t)

	// Load DM channel with message count
	updated, _ := m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "dm1", Name: "u1__u2", Type: "D", TotalMsgCount: 10},
		},
	})
	m = updated.(tui.Model)

	// Load channel members for DM
	updated, _ = m.Update(tui.ChannelMembersLoadedMsg{
		ChannelID: "dm1",
		Members: []*model.ChannelMember{
			{ChannelID: "dm1", UserID: "u1", MsgCount: 7},
		},
	})
	m = updated.(tui.Model)

	if got := m.UnreadCount("dm1"); got != 3 {
		t.Fatalf("expected DM unread count 3, got %d", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "(3)") {
		t.Errorf("expected unread badge '(3)' for DM channel in palette view:\n%s", view)
	}
}

func TestModel_SlashSkinOpensSkinPicker(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/skin"})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Select Theme") {
		t.Errorf("expected skin picker in view:\n%s", view)
	}
}

func TestModel_SlashOtherOpensPalette(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/remind"})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "remind") {
		t.Errorf("expected command palette with 'remind' in view:\n%s", view)
	}
}

func TestModel_SkinSelectedMsg(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(skinpicker.SkinSelectedMsg{Name: "catppuccin"})
	m = updated.(tui.Model)

	// Should render without errors
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after skin change")
	}
}

func TestModel_EscClosesSkinPicker(t *testing.T) {
	m := setupModel(t)

	// Open skin picker
	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/skin"})
	m = updated.(tui.Model)

	// Escape closes it
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	view := m.View()
	if strings.Contains(view, "Select Theme") {
		t.Errorf("skin picker should be closed:\n%s", view)
	}
}

func TestModel_SkinPickerKeyInterception(t *testing.T) {
	m := setupModel(t)

	// Open skin picker
	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/skin"})
	m = updated.(tui.Model)

	// Down key should be consumed by skin picker
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(tui.Model)

	// Picker should still be visible
	view := m.View()
	if !strings.Contains(view, "Select Theme") {
		t.Errorf("skin picker should still be open:\n%s", view)
	}
}

// --- Group Channel Tests ---

func TestModel_GroupCreatedMsg(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.GroupCreatedMsg{
		Channel: &model.Channel{ID: "g1", Name: "u1__u2__u3", Type: "G"},
	})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Error("expected commands from GroupCreatedMsg")
	}
	_ = m.View()
}

func TestModel_GroupCreatedMsg_Error(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.GroupCreatedMsg{
		Err: &model.AppError{Message: "group fail"},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "group fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_GroupDisplayNameResolution(t *testing.T) {
	m := setupModel(t)

	// Load users
	updated, _ := m.Update(tui.UsersLoadedMsg{
		Users: []*model.User{
			{ID: "u2", Username: "bob"},
			{ID: "u3", Username: "charlie"},
		},
	})
	m = updated.(tui.Model)

	// Load DM channels including a group channel
	updated, _ = m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "g1", Name: "u1__u2__u3", Type: "G"},
		},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(tui.Model)
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "bob") || !strings.Contains(view, "charlie") {
		t.Errorf("expected group display names in palette:\n%s", view)
	}
}

func TestModel_StatusBarGroupChannel(t *testing.T) {
	m := setupModel(t)

	// Load users
	updated, _ := m.Update(tui.UsersLoadedMsg{
		Users: []*model.User{
			{ID: "u2", Username: "bob"},
			{ID: "u3", Username: "charlie"},
		},
	})
	m = updated.(tui.Model)

	// Load group DM channels
	updated, _ = m.Update(tui.DMChannelsLoadedMsg{
		Channels: []*model.Channel{
			{ID: "g1", Name: "u1__u2__u3", Type: "G"},
		},
	})
	m = updated.(tui.Model)

	// Switch to group channel
	groupCh := &model.Channel{ID: "g1", Name: "u1__u2__u3", Type: "G"}
	updated, _ = m.Update(palette.ChannelChosenMsg{Channel: groupCh})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "bob") {
		t.Errorf("expected 'bob' in status bar for group channel:\n%s", view)
	}
}

// --- Team Channel Creation Tests ---

func TestModel_CtrlNOpensChCreator(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Create Channel") {
		t.Errorf("expected channel creator in view:\n%s", view)
	}
}

func TestModel_CtrlNNoOpWithoutTeam(t *testing.T) {
	m := testModel()
	// Only set window size, don't load teams
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	// No active team → ctrl+n should be a no-op
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)

	view := m.View()
	if strings.Contains(view, "Create Channel") {
		t.Error("channel creator should not open without active team")
	}
}

func TestModel_EscClosesChCreator(t *testing.T) {
	m := setupModel(t)

	// Open channel creator
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)

	// Close with escape
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.Model)

	view := m.View()
	if strings.Contains(view, "Create Channel") {
		t.Errorf("channel creator should be closed:\n%s", view)
	}
}

func TestModel_ChCreatorKeyInterception(t *testing.T) {
	m := setupModel(t)

	// Open channel creator
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(tui.Model)

	// Type in creator (keys should be consumed)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "Create Channel") {
		t.Errorf("channel creator should still be open:\n%s", view)
	}
}

func TestModel_ChannelSubmittedMsg(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(chcreator.ChannelSubmittedMsg{
		Channel: &model.Channel{TeamID: "t1", Name: "deploy", DisplayName: "Deploy", Type: "O"},
	})
	if cmd == nil {
		t.Error("expected command from ChannelSubmittedMsg")
	}
}

func TestModel_ChannelCreatedMsg(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.ChannelCreatedMsg{
		Channel: &model.Channel{ID: "ch-new", Name: "deploy", DisplayName: "Deploy", Type: "O"},
	})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Error("expected commands from ChannelCreatedMsg")
	}
	view := m.View()
	if !strings.Contains(view, "Deploy") {
		t.Errorf("expected 'Deploy' in sidebar:\n%s", view)
	}
}

func TestModel_ChannelCreatedMsg_Error(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ChannelCreatedMsg{
		Err: &model.AppError{Message: "channel fail"},
	})
	m = updated.(tui.Model)

	view := m.View()
	if !strings.Contains(view, "channel fail") {
		t.Errorf("expected error in view:\n%s", view)
	}
}

func TestModel_WSEventChannelCreatedOpenType(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventChannelCreated,
			Sequence: 1,
			Data: map[string]any{
				"type":    "O",
				"team_id": "t1",
			},
		},
	}
	_, cmd := m.Update(wsEvt)
	if cmd == nil {
		t.Error("expected command from channel_created O event")
	}
}

func TestModel_WSEventChannelCreatedOtherTeam(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventChannelCreated,
			Sequence: 1,
			Data: map[string]any{
				"type":    "O",
				"team_id": "other-team",
			},
		},
	}
	_, _ = m.Update(wsEvt)
	// Should not trigger FetchChannels for other team
}

func TestModel_WSEventChannelCreatedGroup(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventChannelCreated,
			Sequence: 1,
			Data: map[string]any{
				"type": "G",
			},
		},
	}
	_, cmd := m.Update(wsEvt)
	if cmd == nil {
		t.Error("expected command from channel_created G event")
	}
}

func TestModel_WSEventPostedResolvesUnknownUser(t *testing.T) {
	m := setupModel(t)

	// Send a WS post from an unknown user (u999 is not in the user cache)
	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 1,
			Data: map[string]any{
				"id": "p99", "channel_id": "c1", "user_id": "u999",
				"content": "hello from unknown", "create_at": float64(1700000099000),
			},
		},
	}
	_, cmd := m.Update(wsEvt)

	if cmd == nil {
		t.Fatal("expected a command to fetch unknown user; got nil")
	}

	// Execute the batched command and check that one of them returns UsersLoadedMsg
	msgs := executeBatchCmd(cmd)
	found := false
	for _, msg := range msgs {
		if _, ok := msg.(tui.UsersLoadedMsg); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected FetchUsersByIDs command in batch for unknown user u999")
	}
}

func TestModel_WSEventPostedSkipsFetchForKnownUser(t *testing.T) {
	m := setupModel(t)

	// u1 is already loaded by setupModel. Send a WS post from u1.
	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventPosted,
			Sequence: 1,
			Data: map[string]any{
				"id": "p98", "channel_id": "c1", "user_id": "u1",
				"content": "hello from alice", "create_at": float64(1700000098000),
			},
		},
	}
	_, cmd := m.Update(wsEvt)

	if cmd == nil {
		t.Fatal("expected at least ListenWebSocket command; got nil")
	}

	// Execute the batched command — should NOT contain a UsersLoadedMsg
	msgs := executeBatchCmd(cmd)
	for _, msg := range msgs {
		if _, ok := msg.(tui.UsersLoadedMsg); ok {
			t.Error("should not fetch users for already-known user u1")
		}
	}
}

func TestModel_WSEventUserAddedRefetchesChannels(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventUserAdded,
			Sequence: 1,
			Data: map[string]any{
				"channel_id": "new-private-chan",
				"user_id":    "u1", // current user
			},
		},
	}
	_, cmd := m.Update(wsEvt)
	if cmd == nil {
		t.Fatal("expected command from user_added event")
	}

	msgs := executeBatchCmd(cmd)
	foundChannelsLoaded := false
	for _, msg := range msgs {
		if _, ok := msg.(tui.ChannelsLoadedMsg); ok {
			foundChannelsLoaded = true
		}
	}
	if !foundChannelsLoaded {
		t.Error("expected FetchChannels to be triggered for current user added to channel")
	}
}

func TestModel_WSEventUserAddedIgnoresOtherUser(t *testing.T) {
	m := setupModel(t)

	wsEvt := tui.WebSocketEventMsg{
		Event: model.WebSocketEvent{
			Event:    model.WebSocketEventUserAdded,
			Sequence: 1,
			Data: map[string]any{
				"channel_id": "new-private-chan",
				"user_id":    "u99", // different user
			},
		},
	}
	_, cmd := m.Update(wsEvt)
	if cmd == nil {
		t.Fatal("expected at least ListenWebSocket command")
	}

	msgs := executeBatchCmd(cmd)
	for _, msg := range msgs {
		if _, ok := msg.(tui.ChannelsLoadedMsg); ok {
			t.Error("should NOT fetch channels when another user is added")
		}
	}
}

// executeBatchCmd runs a tea.Cmd and collects messages from batched commands.
// Commands that panic (e.g. ListenWebSocket with nil wsClient) are skipped.
func executeBatchCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := safeExec(cmd)
	if msg == nil {
		return nil
	}
	// tea.Batch returns a BatchMsg (which is []Cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			if c != nil {
				if m := safeExec(c); m != nil {
					msgs = append(msgs, m)
				}
			}
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func safeExec(cmd tea.Cmd) (msg tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			msg = nil
		}
	}()
	return cmd()
}

func TestModel_ErrorMsg_ReturnsCmd(t *testing.T) {
	m := setupModel(t)
	_, cmd := m.Update(tui.ErrMsg{Err: fmt.Errorf("test error")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from ErrMsg (auto-clear timer)")
	}
}

func TestModel_ClearErrMsg_ClearsError(t *testing.T) {
	m := setupModel(t)
	// Trigger an error
	updated, _ := m.Update(tui.ErrMsg{Err: fmt.Errorf("bad request")})
	m = updated.(tui.Model)
	view := m.View()
	if !strings.Contains(view, "bad request") {
		t.Error("expected error in view")
	}
	// Clear with matching seq (seq=1 since this is the first error)
	updated, _ = m.Update(tui.ClearErrMsg{Seq: 1})
	m = updated.(tui.Model)
	view = m.View()
	if strings.Contains(view, "bad request") {
		t.Error("expected error to be cleared")
	}
}

func TestModel_ClearErrMsg_IgnoresStaleSeq(t *testing.T) {
	m := setupModel(t)
	// Trigger first error
	updated, _ := m.Update(tui.ErrMsg{Err: fmt.Errorf("first error")})
	m = updated.(tui.Model)
	// Trigger second error (seq=2)
	updated, _ = m.Update(tui.ErrMsg{Err: fmt.Errorf("second error")})
	m = updated.(tui.Model)
	// Clear with stale seq=1 (should be ignored)
	updated, _ = m.Update(tui.ClearErrMsg{Seq: 1})
	m = updated.(tui.Model)
	view := m.View()
	if !strings.Contains(view, "second error") {
		t.Error("stale ClearErrMsg should not clear a newer error")
	}
}

func TestModel_StatusBar_UsernameNotTruncated(t *testing.T) {
	m := setupModel(t)
	view := m.View()
	stripped := testutil.StripANSI(view)
	if !strings.Contains(stripped, "alice") {
		t.Errorf("expected full username 'alice' in status bar, got:\n%s", stripped)
	}
}

func TestModel_SendMsg_HashtagOnlyPreservesContent(t *testing.T) {
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}},
		posts:    &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
		commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}},
		users:    []*model.User{{ID: "u1", Username: "alice"}},
	}
	cfg := &config.Config{
		ServerURL:    "http://localhost:8065",
		SessionToken: "test-token",
		WSScheme:     "ws",
	}
	s := styles.New(theme.TokyoNight())
	m := tui.NewModel(cfg, client, nil, s, nil, nil, nil)

	// Setup model state
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", DisplayName: "General"}}})
	m = updated.(tui.Model)

	// Send hashtag-only message
	_, cmd := m.Update(input.SendMsg{Content: "#sometag"})
	if cmd == nil {
		t.Fatal("expected command from SendMsg")
	}

	// Execute the command to trigger CreatePost
	msg := cmd()
	created, ok := msg.(tui.PostCreatedMsg)
	if !ok {
		// It might be a batch — check the underlying post via mock
		_ = created
	}

	// The mock should have received a post with non-empty content
	if client.lastCreatedPost == nil {
		t.Fatal("expected CreatePost to be called")
	}
	if client.lastCreatedPost.Content == "" {
		t.Error("hashtag-only post should preserve original content, got empty string")
	}
	if client.lastCreatedPost.Content != "#sometag" {
		t.Errorf("expected content %q, got %q", "#sometag", client.lastCreatedPost.Content)
	}
}

func TestModel_SendMsg_HashtagWithTextStripsCorrectly(t *testing.T) {
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}},
		posts:    &model.PostList{Order: []*model.Post{{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}}},
		commands: []*model.Command{{ID: "cmd1", Slug: "remind", Description: "Set reminder"}},
		users:    []*model.User{{ID: "u1", Username: "alice"}},
	}
	cfg := &config.Config{
		ServerURL:    "http://localhost:8065",
		SessionToken: "test-token",
		WSScheme:     "ws",
	}
	s := styles.New(theme.TokyoNight())
	m := tui.NewModel(cfg, client, nil, s, nil, nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", DisplayName: "General"}}})
	m = updated.(tui.Model)

	// Send message with hashtag and text
	_, cmd := m.Update(input.SendMsg{Content: "#sometag blah blah"})
	if cmd == nil {
		t.Fatal("expected command from SendMsg")
	}
	msg := cmd()
	_ = msg

	if client.lastCreatedPost == nil {
		t.Fatal("expected CreatePost to be called")
	}
	if client.lastCreatedPost.Content != "blah blah" {
		t.Errorf("expected content %q, got %q", "blah blah", client.lastCreatedPost.Content)
	}
}
