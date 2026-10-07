package tui_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
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

// modelWithClient builds a sized, logged-in model backed by a specific client,
// so tests can observe the requests it makes.
func modelWithClient(t *testing.T, client api.ChitClient) tui.Model {
	t.Helper()

	cfg := &config.Config{ServerURL: "http://localhost:8065"}
	m := tui.NewModel(cfg, client, nil, styles.New(theme.TokyoNight()), nil, nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	return updated.(tui.Model)
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
	wantMsg[tui.UserLoadedMsg](t, cmd)
	wantMsg[tui.TeamsLoadedMsg](t, cmd)
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
	wantMsg[tui.PostsLoadedMsg](t, cmd)
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
	wantMsg[tui.ThreadLoadedMsg](t, cmd)
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

// Esc closes the palette, whichever mode opened it, and hands the keyboard
// back to the input.
func TestModel_EscClosesThePalette(t *testing.T) {
	for _, opener := range []tea.KeyType{tea.KeyCtrlK, tea.KeyCtrlS} {
		m := setupModel(t)
		m, _ = step(t, m, tea.KeyMsg{Type: opener})
		if m.Focused() != tui.FocusPalette {
			t.Fatalf("%v: palette did not take focus", opener)
		}

		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})

		if m.Focused() != tui.FocusInput {
			t.Errorf("%v: focus = %v after Esc, want the input", opener, m.Focused())
		}
		if strings.Contains(viewOf(m), "↵ select") {
			t.Errorf("%v: palette still drawn after Esc:\n%s", opener, viewOf(m))
		}
	}
}

func TestModel_PostSelectedOpensThread(t *testing.T) {
	m := setupModel(t)

	p := &model.Post{ID: "p1", UserID: "u1", Content: "Hello", CreateAt: 1700000000000}
	_, cmd := m.Update(viewport.PostSelectedMsg{Post: p})
	wantMsg[tui.ThreadLoadedMsg](t, cmd)
}

// Choosing a command inserts it for completing rather than running it.
func TestModel_CommandChosenInsertsIt(t *testing.T) {
	m := setupModel(t)
	if strings.Contains(viewOf(m), "/remind") {
		t.Fatal("setup: /remind already on screen")
	}

	m, _ = step(t, m, palette.CommandChosenMsg{Command: &model.Command{ID: "cmd1", Slug: "remind"}})

	if !strings.Contains(viewOf(m), "/remind") {
		t.Errorf("command not inserted into the input:\n%s", viewOf(m))
	}
	if m.Focused() != tui.FocusInput {
		t.Errorf("focus = %v, want the input", m.Focused())
	}
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

func TestModel_SendPostsToTheActiveChannel(t *testing.T) {
	m, client := withOwnPost(t)

	_ = send(t, m, "test message")

	if p := client.lastCreatedPost; p == nil || p.Content != "test message" || p.ChannelID != "c1" {
		t.Errorf("created %+v, want \"test message\" in c1", p)
	}
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
	wantMsg[tui.PostCreatedMsg](t, cmd)
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

// Commands this client does not implement go to the server as a message;
// the server owns the registry and replies to an unknown one.
func TestModel_ServerSlashCommandIsSent(t *testing.T) {
	m, client := withOwnPost(t)

	_, cmd := step(t, m, input.SlashTriggerMsg{Input: "/remind me in 5m"})
	drain(cmd)

	if p := client.lastCreatedPost; p == nil || p.Content != "/remind me in 5m" {
		t.Errorf("created %+v, want the command sent as typed", p)
	}
}

func TestModel_HistoryKeysMoveTheSelection(t *testing.T) {
	m := setupModel(t)
	// Newest first, as the API returns them.
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: posts("c1", "newest", "oldest")})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got := m.SelectedPostID(); got != "c1-newest" {
		t.Fatalf("setup: selected %q, want the newest", got)
	}

	m, _ = step(t, m, key('k'))

	if got := m.SelectedPostID(); got != "c1-oldest" {
		t.Errorf("selected %q after k, want the post above", got)
	}
}

// Keys reach the input once Tab has cycled focus back to it.
func TestModel_TypingReachesTheInput(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "hi" {
		m, _ = step(t, m, key(r))
	}

	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	for _, msg := range messagesOf(cmd) {
		if sent, ok := msg.(input.SendMsg); ok && sent.Content == "hi" {
			return
		}
	}
	t.Error("Enter did not send what was typed")
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

// Keys typed with the palette open filter it.
func TestModel_TypingFiltersThePalette(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", TeamID: "t1", DisplayName: "General"},
		{ID: "c2", TeamID: "t1", DisplayName: "Random"},
	}})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlK})
	for _, r := range "rand" {
		m, _ = step(t, m, key(r))
	}

	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	for _, msg := range messagesOf(cmd) {
		if chosen, ok := msg.(palette.ChannelChosenMsg); ok {
			if chosen.Channel.ID != "c2" {
				t.Errorf("chose %q, want Random", chosen.Channel.DisplayName)
			}
			return
		}
	}
	t.Error("Enter chose no channel")
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

// Ctrl+S opens the palette in message search: what is typed is searched for.
func TestModel_CtrlSOpensSearch(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.Focused() != tui.FocusPalette {
		t.Fatalf("focus = %v, want the palette", m.Focused())
	}
	for _, r := range "deploy" {
		m, _ = step(t, m, key(r))
	}
	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := wantMsg[palette.SearchSubmitMsg](t, cmd); got.Term != "deploy" {
		t.Errorf("searched for %q, want deploy", got.Term)
	}
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

// Choosing a search hit in this channel selects it in the history.
func TestModel_SearchResultIsSelected(t *testing.T) {
	m := setupModel(t)
	// Newest first: the hit is not the post selected by default.
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: posts("c1", "later", "the hit")})

	m, _ = step(t, m, palette.PostChosenMsg{Post: &model.Post{ID: "c1-the hit", ChannelID: "c1"}})

	if got := m.SelectedPostID(); got != "c1-the hit" {
		t.Errorf("selected %q, want the hit", got)
	}
	if m.Focused() != tui.FocusViewport {
		t.Errorf("focus = %v, want the history pane", m.Focused())
	}
}

func TestModel_SearchSubmitMsg(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(palette.SearchSubmitMsg{Term: "hello"})
	wantMsg[tui.SearchResultsMsg](t, cmd)
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

	// c1 is open, and so always read; c2 has ten messages.
	updated, _ := m.Update(tui.ChannelsLoadedMsg{
		TeamID: "t1",
		Channels: []*model.Channel{
			{ID: "c1", DisplayName: "General"},
			{ID: "c2", DisplayName: "Random", TotalMsgCount: 10},
		},
	})
	m = updated.(tui.Model)

	// Simulate channel members loaded with user having read 7 messages
	updated, _ = m.Update(tui.ChannelMembersLoadedMsg{
		ChannelID: "c2",
		Members: []*model.ChannelMember{
			{ChannelID: "c2", UserID: "u1", MsgCount: 7},
		},
	})
	m = updated.(tui.Model)

	if got := m.UnreadCount("c2"); got != 3 {
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

// A mention counts toward its channel's badge until the channel is viewed.
func TestModel_MentionBadgeCountsUntilViewed(t *testing.T) {
	m := setupModel(t)
	mentioned := tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event:     model.WebSocketEventMentioned,
		Data:      map[string]any{},
		Broadcast: &model.WebSocketBroadcast{ChannelID: "c2"},
	}}

	m, _ = step(t, m, mentioned)
	m, _ = step(t, m, mentioned)
	if got := m.MentionCount("c2"); got != 2 {
		t.Fatalf("mentions = %d after two, want 2", got)
	}

	m, _ = step(t, m, tui.ChannelViewedMsg{ChannelID: "c2"})
	if got := m.MentionCount("c2"); got != 0 {
		t.Errorf("mentions = %d after viewing, want 0", got)
	}
}

func TestModel_MentionPopupShowsAndHides(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, input.AtTriggerMsg{Prefix: "", StartCol: 0})
	if !strings.Contains(viewOf(m), "@channel") {
		t.Fatalf("popup not shown:\n%s", viewOf(m))
	}

	m, _ = step(t, m, input.AtDismissMsg{})
	if strings.Contains(viewOf(m), "@channel") {
		t.Errorf("popup still shown after dismiss:\n%s", viewOf(m))
	}
}

func TestModel_MentionChosenIsInserted(t *testing.T) {
	m := setupModel(t)
	for _, r := range "hi @al" {
		m, _ = step(t, m, key(r))
	}

	m, _ = step(t, m, mention.UserSelectedMsg{Username: "alice", StartCol: 3})

	if !strings.Contains(viewOf(m), "hi @alice") {
		t.Errorf("mention not completed in the input:\n%s", viewOf(m))
	}
}

// The member row for the signed-in user carries their unread mentions.
func TestModel_ChannelMembersSetTheMentionBadge(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", DisplayName: "General"},
		{ID: "c2", DisplayName: "Random"},
	}})

	m, _ = step(t, m, tui.ChannelMembersLoadedMsg{ChannelID: "c2", Members: []*model.ChannelMember{
		{ChannelID: "c2", UserID: "someone-else", MentionCount: 9},
		{ChannelID: "c2", UserID: "u1", MsgCount: 7, MentionCount: 2},
	}})

	if got := m.MentionCount("c2"); got != 2 {
		t.Errorf("mentions = %d, want the signed-in user's 2", got)
	}
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

func TestModel_ThreadLoadedCachesCount(t *testing.T) {
	m := setupModel(t)
	m = startThread(t, m, &model.Post{ID: "p1", UserID: "u1", Content: "Hello"})

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
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "1 replies") {
		t.Errorf("expected '1 replies' badge after ThreadLoadedMsg:\n%s", view)
	}
}

// A failed load says so. Each loader is checked on its own: run together,
// only the last one's message could be seen.
func TestModel_ErrorMsgsFromLoaders(t *testing.T) {
	failed := func(what string) error { return &model.AppError{Message: what + " failed"} }
	tests := map[string]tea.Msg{
		"user":     tui.UserLoadedMsg{Err: failed("user")},
		"teams":    tui.TeamsLoadedMsg{Err: failed("teams")},
		"channels": tui.ChannelsLoadedMsg{Err: failed("channels")},
		"posts":    tui.PostsLoadedMsg{ChannelID: "c1", Err: failed("posts")},
		"send":     tui.PostCreatedMsg{Err: failed("send")},
		"commands": tui.CommandsLoadedMsg{Err: failed("commands")},
		"users":    tui.UsersLoadedMsg{Err: failed("users")},
		"members":  tui.ChannelMembersLoadedMsg{Err: failed("members")},
	}
	for name, msg := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := step(t, setupModel(t), msg)
			if !strings.Contains(viewOf(m), name+" failed") {
				t.Errorf("error not shown:\n%s", viewOf(m))
			}
		})
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
	wantMsg[tui.UserSearchResultsMsg](t, cmd)
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
	wantMsg[tui.DMCreatedMsg](t, cmd)
}

func TestModel_DMCreatedMsg(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.DMCreatedMsg{
		Channel: &model.Channel{ID: "dm1", Name: "u1__u2", Type: "D"},
	})
	m = updated.(tui.Model)

	wantMsg[tui.DMChannelsLoadedMsg](t, cmd)
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
	wantMsg[tui.DMChannelsLoadedMsg](t, cmd)
}

// A channel created in any of the user's teams is picked up; only the active
// team's used to be, so channels in the others never appeared.
func TestModel_ChannelCreatedRefreshesItsTeam(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}, {ID: "t2"}}})
	created := func(team string) tui.WebSocketEventMsg {
		return tui.WebSocketEventMsg{Event: model.WebSocketEvent{
			Event: model.WebSocketEventChannelCreated,
			Data:  map[string]any{"type": "O", "team_id": team},
		}}
	}

	client.channelsFetched = nil
	_, cmd := step(t, m, created("t2"))
	drain(cmd)
	if !slices.Equal(client.channelsFetched, []string{"t2"}) {
		t.Errorf("fetched channels for %v, want [t2]", client.channelsFetched)
	}

	client.channelsFetched = nil
	_, cmd = step(t, m, created("not-mine"))
	drain(cmd)
	if len(client.channelsFetched) != 0 {
		t.Errorf("fetched channels for %v on a team the user is not in", client.channelsFetched)
	}
}

// Being added to a channel says which channel, not which team, so every team
// is refreshed; previously only the active one was.
func TestModel_AddedToAChannelRefreshesEveryTeam(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}, {ID: "t2"}}})
	client.channelsFetched = nil

	_, cmd := step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventUserAdded,
		Data:  map[string]any{"user_id": "u1", "channel_id": "c9"},
	}})
	drain(cmd)

	slices.Sort(client.channelsFetched)
	if !slices.Equal(client.channelsFetched, []string{"t1", "t2"}) {
		t.Errorf("fetched channels for %v, want both teams", client.channelsFetched)
	}
}

// Someone else joining changes who can be @-mentioned in that channel.
func TestModel_SomeoneAddedRefreshesTheActiveChannelsMembers(t *testing.T) {
	m, client := withOwnPost(t)
	client.membersFetched = nil

	_, cmd := step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventUserAdded,
		Data:  map[string]any{"user_id": "u7", "channel_id": "c1"},
	}})
	drain(cmd)

	if !slices.Equal(client.membersFetched, []string{"c1"}) {
		t.Errorf("members fetched for %v, want [c1]", client.membersFetched)
	}
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

	wantMsg[tui.UsersLoadedMsg](t, cmd)

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

// Anything else goes to the server, which owns the command registry and the
// unknown-command reply. Opening the palette here used to discard the text.
func TestModel_SlashCommandIsSentToServer(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(input.SlashTriggerMsg{Input: "/invite bob"})
	m = updated.(tui.Model)

	wantMsg[tui.PostCreatedMsg](t, cmd)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "Commands") {
		t.Errorf("the palette opened instead of sending:\n%s", view)
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
	wantMsg[tui.ChannelCreatedMsg](t, cmd)
}

func TestModel_ChannelCreatedMsg(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.ChannelCreatedMsg{
		Channel: &model.Channel{ID: "ch-new", Name: "deploy", DisplayName: "Deploy", Type: "O"},
	})
	m = updated.(tui.Model)

	wantMsg[tui.PostsLoadedMsg](t, cmd)
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
	wantMsg[tui.ChannelsLoadedMsg](t, cmd)
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
	wantMsg[tui.DMChannelsLoadedMsg](t, cmd)
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

	wantMsg[tui.UsersLoadedMsg](t, cmd)

	// Execute the batched command and check that one of them returns UsersLoadedMsg
	msgs := messagesOf(cmd)
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

	// Execute the batched command — should NOT contain a UsersLoadedMsg
	msgs := messagesOf(cmd)
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

	msgs := messagesOf(cmd)
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

	msgs := messagesOf(cmd)
	for _, msg := range msgs {
		if _, ok := msg.(tui.ChannelsLoadedMsg); ok {
			t.Error("should NOT fetch channels when another user is added")
		}
	}
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
	lines := strings.Split(viewOf(m), "\n")
	if bar := lines[len(lines)-1]; !strings.Contains(bar, "alice") {
		t.Errorf("action bar = %q, want the full username", bar)
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

// The bug this guards: the client intercepted every "/"-prefixed message and
// discarded it, so an ordinary message that happens to start with a slash —
// a path, say — could never be sent, silently. The server's parser rejects
// "/usr/local/bin" as a command, so it must arrive as a normal post.
func TestModel_SlashPrefixedMessageIsNotSwallowed(t *testing.T) {
	tests := []string{
		"/usr/local/bin",
		"/etc/hosts is the file",
		"/",
	}

	for _, content := range tests {
		t.Run(content, func(t *testing.T) {
			m := setupModel(t)

			updated, cmd := m.Update(input.SlashTriggerMsg{Input: content})
			m = updated.(tui.Model)

			if content == "/" {
				// A bare slash browses instead of sending.
				view := testutil.StripANSI(m.View())
				if !strings.Contains(view, "remind") {
					t.Errorf("bare slash should open the palette:\n%s", view)
				}
				return
			}

			if cmd == nil {
				t.Fatalf("%q produced no command; the message was dropped", content)
			}
		})
	}
}

// Choosing a command from the palette used to do nothing at all. It now
// prefills the input so arguments can be typed.
func TestModel_CommandChosenPrefillsInput(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(palette.CommandChosenMsg{
		Command: &model.Command{ID: "cmd1", Slug: "invite"},
	})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "/invite") {
		t.Errorf("expected the input prefilled with /invite:\n%s", view)
	}
}

// The ephemeral reply to a command is broadcast only to the invoker and is
// never persisted, so it had no handler and simply vanished.
func TestModel_CommandResponseIsDisplayed(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data: map[string]any{
			"text":         "bob was added to the channel",
			"channel_id":   "c1",
			"command_slug": "invite",
		},
	}})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "bob was added to the channel") {
		t.Errorf("command response is not shown:\n%s", view)
	}
	if !strings.Contains(view, "/invite") {
		t.Errorf("response is not attributed to the command:\n%s", view)
	}
}

// A response for another channel must not leak into the current one.
func TestModel_CommandResponseForOtherChannelIsIgnored(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data: map[string]any{
			"text":       "secret from elsewhere",
			"channel_id": "other-channel",
		},
	}})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "secret from elsewhere") {
		t.Error("a response for another channel was displayed")
	}
}

// The Reply button is the only on-screen hint that replying exists, and that
// the history pane must be focused first. It appears only when a reply is
// actually possible.
func TestModel_ReplyButtonAppearsWhenHistoryFocused(t *testing.T) {
	m := setupModel(t)

	// Input is focused at startup, so there is nothing to reply to yet.
	if strings.Contains(testutil.StripANSI(m.View()), "Reply") {
		t.Error("Reply button shown while the input is focused")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "Reply") {
		t.Errorf("Reply button missing after focusing history:\n%s",
			testutil.StripANSI(m.View()))
	}
}

// Clicking Reply must take the same path as pressing enter.
func TestModel_ReplyButtonOpensThread(t *testing.T) {
	m := setupModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	col := strings.Index(view[strings.LastIndex(view, "\n")+1:], "Reply")
	if col < 0 {
		t.Fatalf("Reply button not found on the action bar:\n%s", view)
	}

	_, cmd := m.Update(tea.MouseMsg{
		X: col, Y: 39, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft,
	})
	if cmd == nil {
		t.Error("clicking Reply produced no command")
	}
}

// Tagging worked only in the history pane, so a post's tags were unreachable
// once it was open as a thread.
func TestModel_TagPickerOpensFromThread(t *testing.T) {
	m := setupModel(t)

	// Open the thread, then focus its pane — enter opens the thread but
	// leaves focus on the input so a reply can be typed straight away.
	updated, _ := m.Update(viewport.PostSelectedMsg{
		Post: &model.Post{ID: "p1", UserID: "u1", Content: "root"},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.ThreadLoadedMsg{
		PostID: "p1",
		Posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", UserID: "u1", Content: "root", CreateAt: 1700000000000},
		}},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "Filter tags") {
		t.Errorf("tag picker did not open from the thread pane:\n%s",
			testutil.StripANSI(m.View()))
	}
}

// Choosing a search result used to only move focus, leaving the reader
// wherever they already were — which made search results useless.
func TestModel_SearchResultJumpsToPost(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1",
		Posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", UserID: "u1", Content: "first", CreateAt: 1700000000000},
			{ID: "p2", UserID: "u1", Content: "needle here", CreateAt: 1700000001000},
		}},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(palette.PostChosenMsg{Post: &model.Post{ID: "p2"}})
	m = updated.(tui.Model)

	if got := m.SelectedPostID(); got != "p2" {
		t.Errorf("selected post = %q, want p2", got)
	}
}

// A hit outside the loaded window must say so rather than appear to do
// nothing.
func TestModel_SearchResultOutsideHistoryReportsError(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(palette.PostChosenMsg{Post: &model.Post{ID: "ancient"}})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Error("no command returned; the user gets no feedback")
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "older than the loaded history") {
		t.Errorf("no explanation shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// setupModelWithWS is setupModel with a WebSocket client attached.
func setupModelWithWS(t *testing.T) (tui.Model, *mockWSClient) {
	t.Helper()
	wsc := newMockWSClient()
	cfg := &config.Config{ServerURL: "http://localhost:8065"}
	m := tui.NewModel(cfg, &mockClient{}, wsc, styles.New(theme.TokyoNight()), nil, nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.UserLoadedMsg{User: &model.User{ID: "u1", Username: "alice"}})
	return updated.(tui.Model), wsc
}

// The client's channels outlive a sign-out, so the first session's listeners
// serve every later one. Starting a pair per sign-in stacked readers on the
// same stream.
func TestModel_WSListenersStartOnce(t *testing.T) {
	m, _ := setupModelWithWS(t)

	updated, cmd := m.Update(tui.WSConnectedMsg{})
	if cmd == nil {
		t.Fatal("the first connect started no listeners")
	}
	m = updated.(tui.Model)

	if _, cmd := m.Update(tui.WSConnectedMsg{}); cmd != nil {
		t.Error("a second connect started another pair of listeners")
	}
}

// A failed first dial used to be a toast and nothing more: the client never
// retried, and nobody listened for it to recover.
func TestModel_WSFirstDialFailureKeepsListening(t *testing.T) {
	m, _ := setupModelWithWS(t)

	updated, cmd := m.Update(tui.WSConnectedMsg{Err: errors.New("connection refused")})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("no listeners after a failed first dial")
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "offline, retrying") {
		t.Errorf("failed dial not explained:\n%s", view)
	}

	updated, _ = m.Update(tui.WSStateMsg{Connected: true})
	m = updated.(tui.Model)
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "reconnected") {
		t.Errorf("recovery not reported:\n%s", view)
	}
}

func TestModel_WSFirstDialUnauthorizedPromptsLogin(t *testing.T) {
	m, wsc := setupModelWithWS(t)

	updated, _ := m.Update(tui.WSConnectedMsg{Err: fmt.Errorf("ws: %w (HTTP 401)", ws.ErrUnauthorized)})
	m = updated.(tui.Model)
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Chit Login") {
		t.Errorf("rejected credentials did not prompt sign-in:\n%s", view)
	}
	if wsc.closes != 1 {
		t.Errorf("closes = %d, want the client closed once", wsc.closes)
	}
}

// Expiry reported by the socket used to drop the re-armed state listener, so
// the next session never heard about a disconnect.
func TestModel_WSUnauthorizedKeepsTheStateListener(t *testing.T) {
	m, _ := setupModelWithWS(t)
	updated, _ := m.Update(tui.WSConnectedMsg{})
	m = updated.(tui.Model)

	updated, cmd := m.Update(tui.WSStateMsg{Unauthorized: true})
	m = updated.(tui.Model)
	if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
		t.Error("socket expiry did not prompt sign-in")
	}
	if cmd == nil {
		t.Error("the state listener was not re-armed")
	}
}

// Messages from the listeners that land while signed out are stale, but each
// one must re-arm its listener or the next session is deaf.
func TestModel_WSListenersRearmWhileSignedOut(t *testing.T) {
	m, _ := setupModelWithWS(t)
	updated, _ := m.Update(tui.WSConnectedMsg{})
	m = updated.(tui.Model)
	updated, _ = m.Update(input.SlashTriggerMsg{Input: "/logout"})
	m = updated.(tui.Model)
	if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
		t.Fatal("not signed out")
	}

	if _, cmd := m.Update(tui.WebSocketEventMsg{}); cmd == nil {
		t.Error("an event while signed out stopped the event listener")
	}
	if _, cmd := m.Update(tui.WSStateMsg{}); cmd == nil {
		t.Error("a transition while signed out stopped the state listener")
	}
}

// The action-bar indicator used to stay green while the socket was dead:
// reconnect was internal to the ws client and nothing was emitted on drop.
func TestModel_DisconnectIsVisible(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WSConnectedMsg{})
	m = updated.(tui.Model)
	connected := testutil.StripANSI(m.View())

	updated, _ = m.Update(tui.WSStateMsg{Connected: false})
	m = updated.(tui.Model)
	dropped := testutil.StripANSI(m.View())

	if dropped == connected {
		t.Error("the view is identical connected and disconnected")
	}
	if !strings.Contains(dropped, "connection lost") {
		t.Errorf("no disconnect notice shown:\n%s", dropped)
	}
}

// Events during the gap are lost for good, so coming back has to re-read the
// channel rather than leaving a silent hole.
func TestModel_ReconnectBackfillsPosts(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WSStateMsg{Connected: false})
	m = updated.(tui.Model)

	updated, cmd := m.Update(tui.WSStateMsg{Connected: true})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Fatal("reconnecting produced no commands; nothing was refetched")
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "reconnected") {
		t.Errorf("no reconnect notice shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// A state change must re-arm the listener, or the first drop is the last one
// ever reported.
func TestModel_StateListenerReArms(t *testing.T) {
	m, wsc := setupModelWithWS(t)

	_, cmd := m.Update(tui.WSStateMsg{Connected: false})

	// Re-armed means the returned command is waiting on the next transition.
	wsc.state <- ws.ConnState{Connected: true}
	if !slices.ContainsFunc(messagesOf(cmd), func(msg tea.Msg) bool { _, ok := msg.(tui.WSStateMsg); return ok }) {
		t.Error("the state listener was not re-armed")
	}
}

// A full event buffer used to drop events in silence. The socket stays up, so
// no disconnect notice appears and the view simply stops matching the server —
// the one failure mode with nothing at all to reveal it.
func TestModel_DesyncResyncsWhileConnected(t *testing.T) {
	m := setupModel(t)

	// Already connected: this is not a reconnect, and the connected-state
	// transition check would see no change and do nothing.
	updated, _ := m.Update(tui.WSStateMsg{Connected: true})
	m = updated.(tui.Model)

	updated, cmd := m.Update(tui.WSStateMsg{Connected: true, Desynced: true})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Fatal("a desync produced no commands; the stale view was never refetched")
	}
	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "fell behind") {
		t.Errorf("no desync notice shown:\n%s", view)
	}
}

// Nothing reconnected — the socket never dropped — so the notice left on
// screen must be the desync one. Telling the user they reconnected would
// describe an event that did not happen.
func TestModel_DesyncNoticeIsNotAReconnectNotice(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WSStateMsg{Connected: true})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WSStateMsg{Connected: true, Desynced: true})
	m = updated.(tui.Model)

	if view := testutil.StripANSI(m.View()); strings.Contains(view, "reconnected") {
		t.Errorf("desync reported as a reconnect:\n%s", view)
	}
}

// Rejected credentials cannot be fixed by retrying, so they must send the
// user back to the login screen instead of reconnecting forever.
func TestModel_UnauthorizedTriggersReLogin(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WSStateMsg{Connected: false, Unauthorized: true})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
		t.Errorf("expected the login screen:\n%s", testutil.StripANSI(m.View()))
	}
}

// countingClient records how many times each endpoint is called, so the
// request-count guarantees below are asserted rather than assumed.
type countingClient struct {
	*mockClient
	tagBatchCalls  int
	tagSingleCalls int
	memberCalls    int
}

func (c *countingClient) GetTagsForPosts(ctx context.Context, ids []string) (map[string][]*model.Tag, error) {
	c.tagBatchCalls++
	return c.mockClient.GetTagsForPosts(ctx, ids)
}

func (c *countingClient) GetTagsForPost(ctx context.Context, id string) ([]*model.Tag, error) {
	c.tagSingleCalls++
	return c.mockClient.GetTagsForPost(ctx, id)
}

func (c *countingClient) GetChannelMembers(ctx context.Context, id string) ([]*model.ChannelMember, error) {
	c.memberCalls++
	return c.mockClient.GetChannelMembers(ctx, id)
}

// Loading a page of history used to issue one tag request per post — sixty
// per channel open, against a server limited to 10 rps with a burst of 50.
func TestModel_ChannelLoadIssuesOneTagRequest(t *testing.T) {
	client := &countingClient{mockClient: &mockClient{}}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})

	posts := make([]*model.Post, 0, 60)
	for i := range 60 {
		posts = append(posts, &model.Post{
			ID: fmt.Sprintf("p%d", i), UserID: "u1",
			Content: "hi", CreateAt: 1700000000000,
		})
	}

	_, cmd := m.Update(tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: posts}})
	drain(cmd)

	if client.tagSingleCalls != 0 {
		t.Errorf("per-post tag requests = %d, want 0", client.tagSingleCalls)
	}
	if client.tagBatchCalls != 1 {
		t.Errorf("batched tag requests = %d, want exactly 1", client.tagBatchCalls)
	}
}

// Members are only ever read for the active channel, so loading the channel
// list must not fetch them for every channel in every team.
func TestModel_ChannelListDoesNotFetchAllMembers(t *testing.T) {
	client := &countingClient{mockClient: &mockClient{}}
	m := modelWithClient(t, client)

	channels := make([]*model.Channel, 0, 30)
	for i := range 30 {
		channels = append(channels, &model.Channel{
			ID: fmt.Sprintf("c%d", i), DisplayName: fmt.Sprintf("Channel %d", i),
		})
	}

	_, cmd := m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: channels})
	drain(cmd)

	// At most the one channel auto-selected on load.
	if client.memberCalls > 1 {
		t.Errorf("member requests = %d for %d channels, want at most 1",
			client.memberCalls, len(channels))
	}
}

// drain runs a command tree to completion so the requests it issues are
// counted. tea.Batch returns its children as a BatchMsg.
func drain(cmd tea.Cmd) {
	for range messagesOf(cmd) {
	}
}

// wantMsg runs cmd and returns its first message of type T, failing the
// test if there is none. A command merely existing proves little: setError
// alone returns one, for its auto-clear timer.
func wantMsg[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	for _, msg := range messagesOf(cmd) {
		if v, ok := msg.(T); ok {
			return v
		}
	}
	var zero T
	t.Fatalf("no %T among the command's messages", zero)
	return zero
}

// messagesOf runs a command and returns its messages, unwrapping batches.
// A command that does not return promptly is a timer, such as the status
// line's ten-second auto-clear, and is abandoned rather than waited out:
// nothing a test checks takes that long.
func messagesOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(100 * time.Millisecond):
		return nil
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, messagesOf(c)...)
	}
	return out
}

// A sent message used to appear only via the WebSocket echo, so with the
// socket down the input cleared and the message vanished.
func TestModel_OwnPostAppearsWithoutTheEcho(t *testing.T) {
	m := setupModel(t)

	post := &model.Post{ID: "new1", ChannelID: "c1", UserID: "u1",
		Content: "sent while offline", CreateAt: 1700000009000}

	updated, _ := m.Update(tui.PostCreatedMsg{Post: post})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "sent while offline") {
		t.Errorf("own post not shown without the echo:\n%s", testutil.StripANSI(m.View()))
	}
}

// When the echo does arrive it must not duplicate the message.
func TestModel_EchoDoesNotDuplicateOwnPost(t *testing.T) {
	m := setupModel(t)

	post := &model.Post{ID: "new1", ChannelID: "c1", UserID: "u1",
		Content: "only once", CreateAt: 1700000009000}

	updated, _ := m.Update(tui.PostCreatedMsg{Post: post})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data: map[string]any{
			"id": "new1", "channel_id": "c1", "user_id": "u1",
			"content": "only once", "create_at": 1700000009000,
		},
	}})
	m = updated.(tui.Model)

	if n := strings.Count(testutil.StripANSI(m.View()), "only once"); n != 1 {
		t.Errorf("message appears %d times, want 1", n)
	}
}

// The synthetic post returned for a slash command is delivered separately as
// an ephemeral event; appending it here would show it twice.
func TestModel_CommandResponsePostIsNotAppended(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostCreatedMsg{Post: &model.Post{
		ID: "cr1", ChannelID: "c1", UserID: "u1",
		Type: "command_response", Content: "ephemeral reply",
	}})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "ephemeral reply") {
		t.Error("the command-response post was appended to the history")
	}
}

// Ten of the fifteen declared event types had no handler, so the UI silently
// drifted out of step with the server until a reload.
func TestModel_PostEditedUpdatesInPlace(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostEdited,
		Data: map[string]any{
			"id": "p1", "channel_id": "c1", "user_id": "u1",
			"content": "edited text", "create_at": 1700000000000,
		},
	}})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "edited text") {
		t.Errorf("edit not applied:\n%s", view)
	}
	if strings.Contains(view, "Hello") {
		t.Errorf("the old text is still shown:\n%s", view)
	}
}

func TestModel_PostDeletedRemovesIt(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostDeleted,
		Data:  map[string]any{"post_id": "p1", "channel_id": "c1"},
	}})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "Hello") {
		t.Errorf("deleted post is still shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// A rename should appear without a reload.
func TestModel_ChannelUpdatedRenames(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventChannelUpdated,
		Data:  map[string]any{"id": "c1", "display_name": "Renamed Channel"},
	}})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "Renamed Channel") {
		t.Errorf("rename not shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// Being removed from the channel you are reading has to move you off it,
// rather than leaving a view you can no longer refresh.
func TestModel_RemovedFromActiveChannel(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventUserRemoved,
		Data:  map[string]any{"channel_id": "c1", "user_id": "u1"},
	}})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "no longer available") {
		t.Errorf("no explanation shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// Someone else leaving only invalidates the cached member list.
func TestModel_OtherUserRemovedRefetchesMembers(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventUserRemoved,
		Data:  map[string]any{"channel_id": "c1", "user_id": "someone-else"},
	}})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Error("no refetch issued; the mention list stays stale")
	}
	if strings.Contains(testutil.StripANSI(m.View()), "no longer available") {
		t.Error("another user leaving should not move me off the channel")
	}
}

func TestModel_ChannelDeletedMovesAway(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventChannelDeleted,
		Data:  map[string]any{"channel_id": "c1"},
	}})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "no longer available") {
		t.Errorf("no explanation shown:\n%s", testutil.StripANSI(m.View()))
	}
}

func TestModel_EditLoadsPostIntoInput(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab}) // focus history

	m, _ = step(t, m, key('e'))

	// Once in the history, once more in the input.
	if n := strings.Count(viewOf(m), "Hello"); n != 2 {
		t.Errorf("post text shown %d times, want it loaded into the input too:\n%s", n, viewOf(m))
	}
}

func TestModel_SendWhileEditingUpdatesThePost(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(tui.Model)

	updated, cmd := m.Update(input.SendMsg{Content: "revised text"})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("no command issued for the edit")
	}

	// Run it and feed the result back, as the runtime would.
	if edited, ok := cmd().(tui.PostEditedMsg); ok {
		updated, _ = m.Update(edited)
		m = updated.(tui.Model)
	} else {
		t.Fatalf("expected PostEditedMsg, got %T", cmd())
	}

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "revised text") {
		t.Errorf("edit not applied:\n%s", view)
	}
}

// The server refuses edits from anyone but the author, so the keys must be
// inert on other people's messages rather than producing an error.
func TestModel_CannotEditSomeoneElsesPost(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1",
		Posts: &model.PostList{Order: []*model.Post{
			{ID: "p9", UserID: "someone-else", Content: "not mine", CreateAt: 1700000000000},
		}},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(tui.Model)

	if cmd != nil {
		t.Error("delete was issued for another user's post")
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "not mine") {
		t.Error("the post was removed locally despite not being ours")
	}
}

func TestModel_DeleteRemovesOwnPost(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostDeletedMsg{PostID: "p1"})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "Hello") {
		t.Errorf("post still shown after delete:\n%s", testutil.StripANSI(m.View()))
	}
}

// fullPage builds a page of history of the size the client asks for.
func fullPage(t *testing.T, prefix string, n int) []*model.Post {
	t.Helper()
	posts := make([]*model.Post, 0, n)
	for i := range n {
		posts = append(posts, &model.Post{
			ID: fmt.Sprintf("%s%d", prefix, i), ChannelID: "c1", UserID: "u1",
			Content:  fmt.Sprintf("%s message %d", prefix, i),
			CreateAt: int64(1700000000000 + i*1000),
		})
	}
	return posts
}

// History was capped at the first page, so older messages were unreachable.
func TestModel_ScrollingToTopLoadsOlderPosts(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1", Posts: &model.PostList{Order: fullPage(t, "recent", 60)},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	// Scroll to the very top.
	var cmd tea.Cmd
	for range 200 {
		updated, cmd = m.Update(tea.MouseMsg{
			X: 5, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp,
		})
		m = updated.(tui.Model)
		if cmd != nil {
			break
		}
	}

	if cmd == nil {
		t.Fatal("reaching the top issued no fetch; older history is unreachable")
	}
	// The fetch is batched with a "loading older" notice whose auto-clear is
	// a ten-second tick, so the batch is inspected rather than run.
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch, got %T", cmd())
	}
	var loaded *tui.OlderPostsLoadedMsg
	for _, c := range batch {
		if msg, ok := c().(tui.OlderPostsLoadedMsg); ok {
			loaded = &msg
			break
		}
	}
	if loaded == nil {
		t.Fatal("no OlderPostsLoadedMsg produced")
	}
	if loaded.Page != 1 {
		t.Errorf("requested page %d, want 1", loaded.Page)
	}
}

// Older posts go before the loaded ones, and the reader stays put.
func TestModel_OlderPostsArePrepended(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1", Posts: &model.PostList{Order: fullPage(t, "recent", 60)},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.OlderPostsLoadedMsg{
		ChannelID: "c1", Page: 1,
		Posts: &model.PostList{Order: fullPage(t, "older", 60)},
	})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "recent message") {
		t.Error("the current page was replaced instead of extended")
	}
}

// A short page means the server has nothing older, so stop asking.
func TestModel_ShortPageStopsPaging(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1", Posts: &model.PostList{Order: fullPage(t, "recent", 3)},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	for range 50 {
		var cmd tea.Cmd
		updated, cmd = m.Update(tea.MouseMsg{
			X: 5, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp,
		})
		m = updated.(tui.Model)
		if cmd != nil {
			t.Fatal("kept paging after a short first page")
		}
	}
}

// A page that arrives after the reader has moved on must not be spliced into
// the channel they are now looking at.
func TestModel_OlderPostsForAnotherChannelIgnored(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1", Posts: &model.PostList{Order: fullPage(t, "recent", 60)},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.OlderPostsLoadedMsg{
		ChannelID: "other", Page: 1,
		Posts: &model.PostList{Order: fullPage(t, "elsewhere", 60)},
	})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "elsewhere message") {
		t.Error("a page for another channel was spliced in")
	}
}

// Failing to mark a channel read leaves its badge disagreeing with what the
// reader just did; it used to say nothing at all.
func TestModel_ChannelViewedErrorIsReported(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ChannelViewedMsg{
		ChannelID: "c1", Err: errors.New("boom"),
	})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "mark the channel read") {
		t.Errorf("no explanation shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// Tags that silently never load look like a post that has none.
func TestModel_TagLoadErrorIsReported(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostTagsLoadedMsg{
		PostID: "p1", Err: errors.New("boom"),
	})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "load tags") {
		t.Errorf("no explanation shown:\n%s", testutil.StripANSI(m.View()))
	}
}

// A payload the client cannot read must not look like a message that never
// arrived; it is dropped, but not silently.
func TestModel_UndecodablePostedEventDoesNotCrash(t *testing.T) {
	m, wsc := setupModelWithWS(t)

	_, cmd := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPosted,
		Data:  map[string]any{"id": 12345}, // wrong type for a string field
	}})

	// Re-armed means the returned command is waiting on the next event.
	wsc.events <- model.WebSocketEvent{Event: model.WebSocketEventPosted}
	if !slices.ContainsFunc(messagesOf(cmd), func(msg tea.Msg) bool { _, ok := msg.(tui.WebSocketEventMsg); return ok }) {
		t.Error("the listener was not re-armed after an undecodable event")
	}
}

// Session expiry used to be checked on only five message types, so any other
// request noticing it showed a transient toast and left the user in a client
// that could no longer reach the server.
func TestModel_SessionExpiryFromAnyRequest(t *testing.T) {
	expired := &api.APIError{StatusCode: 401}

	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{name: "thread load", msg: tui.ThreadLoadedMsg{Err: expired}},
		{name: "search", msg: tui.SearchResultsMsg{Err: expired}},
		{name: "tag load", msg: tui.PostTagsLoadedMsg{Err: expired}},
		{name: "channel members", msg: tui.ChannelMembersLoadedMsg{Err: expired}},
		{name: "mark read", msg: tui.ChannelViewedMsg{Err: expired}},
		{name: "delete", msg: tui.PostDeletedMsg{Err: expired}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := setupModel(t)

			updated, _ := m.Update(tc.msg)
			m = updated.(tui.Model)

			if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
				t.Errorf("expiry noticed by %s did not prompt re-login:\n%s",
					tc.name, testutil.StripANSI(m.View()))
			}
		})
	}
}

// Pinning is a channel-level act, so unlike editing it works on anyone's post.
func TestModel_PinWorksOnAnyPost(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.PostsLoadedMsg{
		ChannelID: "c1",
		Posts: &model.PostList{Order: []*model.Post{
			{ID: "p9", UserID: "someone-else", Content: "not mine", CreateAt: 1700000000000},
		}},
	})
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("pin issued no command for another user's post")
	}
	if msg, ok := cmd().(tui.PostPinnedMsg); !ok {
		t.Errorf("expected PostPinnedMsg, got %T", cmd())
	} else if !msg.Pinned {
		t.Error("pinning an unpinned post should pin it")
	}
}

// The pin events carry only {post_id, channel_id}, not the whole post — a
// different shape from post_edited. Testing with the wrong shape hid this.
func TestModel_PinEventUsesPostIDPayload(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostPinned,
		Data:  map[string]any{"post_id": "p1", "channel_id": "c1"},
	}})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "[pinned]") {
		t.Errorf("pin badge not shown:\n%s", testutil.StripANSI(m.View()))
	}

	updated, _ = m.Update(tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostUnpinned,
		Data:  map[string]any{"post_id": "p1", "channel_id": "c1"},
	}})
	m = updated.(tui.Model)

	if strings.Contains(testutil.StripANSI(m.View()), "[pinned]") {
		t.Errorf("pin badge still shown after unpin:\n%s", testutil.StripANSI(m.View()))
	}
}

// A global result usually lives in another channel, so choosing it has to
// open that channel first — the post is not in the loaded history until then.
func TestModel_GlobalSearchResultSwitchesChannel(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ChannelsLoadedMsg{
		TeamID: "t1",
		Channels: []*model.Channel{
			{ID: "c1", DisplayName: "General"},
			{ID: "c2", DisplayName: "Elsewhere"},
		},
	})
	m = updated.(tui.Model)

	updated, cmd := m.Update(palette.PostChosenMsg{
		Post: &model.Post{ID: "far", ChannelID: "c2"},
	})
	m = updated.(tui.Model)

	if cmd == nil {
		t.Fatal("choosing a result in another channel did nothing")
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "opened the channel") {
		t.Errorf("no explanation for the channel switch:\n%s", testutil.StripANSI(m.View()))
	}
}

func TestModel_SearchEverywhereUsesTheGlobalEndpoint(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(palette.SearchSubmitMsg{Term: "needle", Everywhere: true})
	if cmd == nil {
		t.Fatal("no search issued")
	}
	if _, ok := cmd().(tui.SearchResultsMsg); !ok {
		t.Errorf("expected SearchResultsMsg, got %T", cmd())
	}
}

// Searching everywhere must work even with no channel open, which is exactly
// when you cannot name the channel you want.
func TestModel_SearchEverywhereWithoutActiveChannel(t *testing.T) {
	m := testModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	_, cmd := m.Update(palette.SearchSubmitMsg{Term: "needle", Everywhere: true})
	if cmd == nil {
		t.Error("global search requires an active channel, which defeats the point")
	}
}

// Leaving a channel had no client path at all, despite the endpoint existing.
func TestModel_LeaveChannel(t *testing.T) {
	m := setupModel(t)

	_, cmd := m.Update(input.SlashTriggerMsg{Input: "/leave"})

	if cmd == nil {
		t.Fatal("/leave issued no request")
	}
	if _, ok := cmd().(tui.ChannelLeftMsg); !ok {
		t.Errorf("expected ChannelLeftMsg, got %T", cmd())
	}
}

// Leaving must drop the channel locally rather than waiting on the
// user_removed broadcast, which only arrives if the socket is up.
func TestModel_LeftChannelIsRemovedLocally(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ChannelLeftMsg{ChannelID: "c1"})
	m = updated.(tui.Model)

	if !strings.Contains(testutil.StripANSI(m.View()), "no longer available") {
		t.Errorf("no explanation after leaving:\n%s", testutil.StripANSI(m.View()))
	}
}

// groupRecordingClient captures the membership sent to CreateGroupChannel. The
// shared mockClient discards it, and the membership is the whole substance of
// a group channel — a test that ignored it would pass on an empty group.
type groupRecordingClient struct {
	*mockClient
	gotIDs  []string
	calls   int
	channel *model.Channel
}

func (c *groupRecordingClient) CreateGroupChannel(_ context.Context, userIDs []string) (*model.Channel, error) {
	c.calls++
	c.gotIDs = append([]string(nil), userIDs...)
	return c.channel, nil
}

func newGroupModel(t *testing.T) (tui.Model, *groupRecordingClient) {
	t.Helper()

	client := &groupRecordingClient{
		mockClient: &mockClient{me: &model.User{ID: "u1", Username: "alice"}},
		channel:    &model.Channel{ID: "g1", Type: model.ChannelGroup, DisplayName: "alice, bob, chad"},
	}
	return modelWithClient(t, client), client
}

// openGroupPicker runs /group through the input, the way a user reaches it.
func openGroupPicker(t *testing.T, m tui.Model) tui.Model {
	t.Helper()

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/group"})
	return updated.(tui.Model)
}

// CreateGroupChannel was implemented, tested, and reachable over REST, but
// nothing in the UI ever called it — group channels could not be created from
// the TUI at all.
func TestModel_GroupCommandCreatesAGroupChannel(t *testing.T) {
	m, client := newGroupModel(t)
	m = openGroupPicker(t, m)

	_, cmd := m.Update(dmpicker.MembersPickedMsg{Users: []*model.User{
		{ID: "u2", Username: "bob"},
		{ID: "u3", Username: "chad"},
	}})

	if cmd == nil {
		t.Fatal("picking members produced no command; no group was created")
	}
	cmd() // run the command so the client is actually called

	if client.calls != 1 {
		t.Fatalf("CreateGroupChannel called %d times, want 1", client.calls)
	}
	want := []string{"u1", "u2", "u3"}
	if !slices.Equal(client.gotIDs, want) {
		t.Errorf("members = %v, want %v", client.gotIDs, want)
	}
}

// The server rejects a group that leaves out its creator, so the caller has to
// be added rather than assumed — and added exactly once even when the picker
// offers them back.
func TestModel_GroupIncludesTheCallerExactlyOnce(t *testing.T) {
	m, client := newGroupModel(t)
	m = openGroupPicker(t, m)

	_, cmd := m.Update(dmpicker.MembersPickedMsg{Users: []*model.User{
		{ID: "u1", Username: "alice"}, // the caller, picked from their own results
		{ID: "u2", Username: "bob"},
		{ID: "u3", Username: "chad"},
	}})
	if cmd == nil {
		t.Fatal("picking members produced no command")
	}
	cmd()

	if got := strings.Count(strings.Join(client.gotIDs, ","), "u1"); got != 1 {
		t.Errorf("caller appears %d times in %v, want exactly 1", got, client.gotIDs)
	}
}

// Two people are a DM, which has its own flow. Sending it anyway would earn a
// server rejection the user cannot act on.
func TestModel_GroupBelowThreeIsRefusedLocally(t *testing.T) {
	m, client := newGroupModel(t)
	m = openGroupPicker(t, m)

	// Deliberately not run: the refusal path schedules the status-bar clear,
	// a ten-second tick that would stall the suite. The notice below is what
	// distinguishes a local refusal from a silent one.
	updated, _ := m.Update(dmpicker.MembersPickedMsg{Users: []*model.User{
		{ID: "u2", Username: "bob"},
	}})
	m = updated.(tui.Model)

	if client.calls != 0 {
		t.Errorf("CreateGroupChannel called with %v; a pair is a DM", client.gotIDs)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "at least three") {
		t.Errorf("no explanation shown:\n%s", view)
	}
}

// A group channel is only its members, so abandoning the pick leaves nothing
// to create — unlike a private channel, which is already named by then.
func TestModel_AbandonedGroupPickCreatesNothing(t *testing.T) {
	m, client := newGroupModel(t)
	m = openGroupPicker(t, m)

	updated, cmd := m.Update(dmpicker.CancelledMsg{})
	m = updated.(tui.Model)
	if cmd != nil {
		cmd()
	}

	if client.calls != 0 {
		t.Errorf("dismissing the picker still created a group with %v", client.gotIDs)
	}

	// And the abandoned intent must not leak into the next pick, which
	// belongs to whatever opened the picker after it.
	_, cmd = m.Update(dmpicker.MembersPickedMsg{Users: []*model.User{
		{ID: "u2"}, {ID: "u3"},
	}})
	if cmd != nil {
		cmd()
	}
	if client.calls != 0 {
		t.Errorf("an abandoned /group still created a group on the next pick: %v", client.gotIDs)
	}
}

// PUT /users/me existed server-side with no client counterpart, so a display
// name could not be changed from the TUI at all.
func TestModel_NickChangesTheDisplayName(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice", DisplayName: "Alice"}}
	m := modelWithClient(t, client)

	_, cmd := m.Update(input.SlashTriggerMsg{Input: "/nick Alice Anderson"})
	if cmd == nil {
		t.Fatal("/nick produced no command")
	}
	msg, ok := cmd().(tui.ProfileUpdatedMsg)
	if !ok {
		t.Fatalf("expected a profile update, got %T", cmd())
	}
	if msg.Err != nil {
		t.Fatalf("update failed: %v", msg.Err)
	}
	// The whole name, not just the first word — display names have spaces.
	if msg.User.DisplayName != "Alice Anderson" {
		t.Errorf("display name = %q, want %q", msg.User.DisplayName, "Alice Anderson")
	}
	if msg.User.Username != "alice" {
		t.Errorf("/nick changed the username to %q; it must only touch the display name",
			msg.User.Username)
	}
}

func TestModel_UsernameChangesTheHandle(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice", DisplayName: "Alice"}}
	m := modelWithClient(t, client)

	_, cmd := m.Update(input.SlashTriggerMsg{Input: "/username alicea"})
	if cmd == nil {
		t.Fatal("/username produced no command")
	}
	msg, ok := cmd().(tui.ProfileUpdatedMsg)
	if !ok {
		t.Fatalf("expected a profile update, got %T", cmd())
	}
	if msg.User.Username != "alicea" {
		t.Errorf("username = %q, want %q", msg.User.Username, "alicea")
	}
	if msg.User.DisplayName != "Alice" {
		t.Errorf("/username cleared the display name to %q", msg.User.DisplayName)
	}
}

// A saved profile has to reach the parts of the view rendered from it, or the
// old name shows until the next sign-in.
func TestModel_ProfileUpdateRefreshesTheView(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(tui.ProfileUpdatedMsg{
		User: &model.User{ID: "u1", Username: "alicea", DisplayName: "Alice Anderson"},
	})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "alicea") {
		t.Errorf("the new username is not shown anywhere:\n%s", view)
	}
	if strings.Contains(view, "profile updated") == false {
		t.Errorf("no confirmation shown:\n%s", view)
	}
}

// An empty argument is a mistake, not a request to blank the field — and the
// server would silently ignore it, leaving nothing to explain the no-op.
func TestModel_ProfileCommandsRefuseEmptyArguments(t *testing.T) {
	for _, cmdText := range []string{"/nick", "/username", "/nick   "} {
		m := setupModel(t)
		updated, _ := m.Update(input.SlashTriggerMsg{Input: cmdText})
		m = updated.(tui.Model)

		if view := testutil.StripANSI(m.View()); !strings.Contains(view, "usage:") {
			t.Errorf("%q gave no usage hint:\n%s", cmdText, view)
		}
	}
}

// A username with a space is never valid, and the mistake is almost always a
// display name typed into the wrong command.
func TestModel_UsernameRejectsSpaces(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice"}}
	m := modelWithClient(t, client)

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/username Alice Anderson"})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "cannot contain spaces") {
		t.Errorf("no explanation shown:\n%s", view)
	}
	if !strings.Contains(view, "/nick") {
		t.Errorf("does not point at the command they meant:\n%s", view)
	}
}

func inboxThread(rootID, channelID, content string, lastReply, lastViewed int64) *model.ThreadResponse {
	return &model.ThreadResponse{
		Thread: &model.Thread{
			PostID: rootID, ChannelID: channelID, ReplyCount: 2, LastReplyAt: lastReply,
		},
		Posts:        []*model.Post{{ID: rootID, ChannelID: channelID, Content: content}},
		LastViewedAt: lastViewed,
	}
}

// The server has served a followed-thread list since the beginning; nothing in
// the client ever asked for it.
func TestModel_ThreadsCommandOpensTheInbox(t *testing.T) {
	m := setupModel(t)

	updated, cmd := m.Update(input.SlashTriggerMsg{Input: "/threads"})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("/threads produced no command; the list was never fetched")
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Threads you follow") {
		t.Errorf("the inbox did not open:\n%s", view)
	}

	updated, _ = m.Update(tui.ThreadsLoadedMsg{
		Threads: []*model.ThreadResponse{inboxThread("p9", "c1", "the deploy is stuck", 300, 100)},
	})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "the deploy is stuck") {
		t.Errorf("the loaded thread is not listed:\n%s", view)
	}
	if !strings.Contains(view, "#General") {
		t.Errorf("the row does not say which channel it is in:\n%s", view)
	}
}

// Opening a thread from the inbox has to switch to its channel: the thread
// pane renders against the active channel, and the thread is often somewhere
// the user is not currently looking.
func TestModel_OpeningAnInboxThreadSwitchesChannel(t *testing.T) {
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		posts:    &model.PostList{},
		thread:   &model.PostList{},
		channels: []*model.Channel{{ID: "c1", DisplayName: "General"}, {ID: "c2", DisplayName: "Design"}},
	}
	m := modelWithClient(t, client)

	updated, _ := m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Eng"}}})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ChannelsLoadedMsg{TeamID: "t1", Channels: client.channels})
	m = updated.(tui.Model)

	updated, cmd := m.Update(threadinbox.ThreadChosenMsg{RootID: "p9", ChannelID: "c2"})
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("choosing a thread produced no command")
	}
	drain(cmd)

	if !slices.Contains(client.readThreads, "p9") {
		t.Errorf("opening the thread did not mark it read; readThreads = %v", client.readThreads)
	}
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Design") {
		t.Errorf("did not switch to the thread's channel:\n%s", view)
	}
}

// Unfollowing has to reach the server. Removing the row alone would put the
// thread back on the next fetch.
func TestModel_UnfollowFromTheInboxReachesTheServer(t *testing.T) {
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice"}}
	m := modelWithClient(t, client)

	updated, _ := m.Update(tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Eng"}}})
	m = updated.(tui.Model)

	_, cmd := m.Update(threadinbox.FollowToggledMsg{RootID: "p9", Following: false})
	if cmd == nil {
		t.Fatal("unfollowing produced no command")
	}
	drain(cmd)

	if len(client.followCalls) != 1 {
		t.Fatalf("follow calls = %v, want one", client.followCalls)
	}
	if client.followCalls[0].RootID != "p9" || client.followCalls[0].Following {
		t.Errorf("sent %+v, want p9 with following=false", client.followCalls[0])
	}
}

// A failed fetch must leave the loading state, or the overlay says "Loading…"
// forever with the reason hidden.
func TestModel_ThreadFetchFailureIsReported(t *testing.T) {
	m := setupModel(t)

	updated, _ := m.Update(input.SlashTriggerMsg{Input: "/threads"})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.ThreadsLoadedMsg{Err: errors.New("boom")})
	m = updated.(tui.Model)

	view := testutil.StripANSI(m.View())
	if strings.Contains(view, "Loading") {
		t.Errorf("still loading after the fetch failed:\n%s", view)
	}
	if !strings.Contains(view, "boom") {
		t.Errorf("the failure is not reported:\n%s", view)
	}
}
