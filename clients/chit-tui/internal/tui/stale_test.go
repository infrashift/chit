package tui_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

func posts(channelID string, contents ...string) *model.PostList {
	pl := &model.PostList{}
	for i, c := range contents {
		pl.Order = append(pl.Order, &model.Post{
			ID: channelID + "-" + c, ChannelID: channelID, UserID: "u1",
			Content: c, CreateAt: int64(1700000000000 + i*1000),
		})
	}
	return pl
}

func viewOf(m tui.Model) string { return testutil.StripANSI(m.View()) }

// Switching channel quickly used to let the first channel's history, landing
// late, replace the second's under the second's name.
func TestModel_LatePostsForAnEarlierChannelAreIgnored(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", TeamID: "t1", DisplayName: "Random"}})

	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: posts("c1", "late from general")})
	if strings.Contains(viewOf(m), "late from general") {
		t.Fatalf("an earlier channel's history replaced this one:\n%s", viewOf(m))
	}

	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c2", Posts: posts("c2", "from random")})
	if !strings.Contains(viewOf(m), "from random") {
		t.Errorf("the open channel's history was not shown:\n%s", viewOf(m))
	}
}

func startThread(t *testing.T, m tui.Model, root *model.Post) tui.Model {
	t.Helper()
	m, _ = step(t, m, viewport.PostSelectedMsg{Post: root})
	return m
}

func threadOf(root string, replies ...string) *model.PostList {
	pl := &model.PostList{Order: []*model.Post{{ID: root, ChannelID: "c1", UserID: "u1", Content: root + " root", CreateAt: 1700000000000}}}
	for i, r := range replies {
		pl.Order = append(pl.Order, &model.Post{ID: r, ChannelID: "c1", UserID: "u1", RootID: root, Content: r, CreateAt: int64(1700000001000 + i)})
	}
	return pl
}

// A thread opened earlier, landing late, replaced the one being read.
func TestModel_LateThreadIsIgnored(t *testing.T) {
	m := setupModel(t)
	m = startThread(t, m, &model.Post{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "Hello"})

	m, _ = step(t, m, tui.ThreadLoadedMsg{PostID: "elsewhere", Posts: threadOf("elsewhere", "wrong reply")})
	if strings.Contains(viewOf(m), "wrong reply") {
		t.Errorf("another thread replaced the open one:\n%s", viewOf(m))
	}
}

// A reply typed before the thread finished loading went to the channel as a
// top-level post. The root's ID is known from the moment the thread opens.
func TestModel_ReplyBeforeTheThreadLoadsIsStillAReply(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})
	m = startThread(t, m, &model.Post{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "Hello"})

	_, cmd := step(t, m, input.SendMsg{Content: "quick reply"})
	drain(cmd)

	if client.lastCreatedPost == nil {
		t.Fatal("nothing was sent")
	}
	if client.lastCreatedPost.RootID != "p1" {
		t.Errorf("RootID = %q, want the open thread p1", client.lastCreatedPost.RootID)
	}
}

// A failed thread load left an empty thread pane, and anything typed into it
// was posted to the channel.
func TestModel_ThreadLoadErrorClosesThePane(t *testing.T) {
	m := setupModel(t)
	m = startThread(t, m, &model.Post{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "Hello"})

	m, _ = step(t, m, tui.ThreadLoadedMsg{PostID: "p1", Err: errors.New("thread gone")})

	view := viewOf(m)
	if !strings.Contains(view, "thread gone") {
		t.Errorf("error not shown:\n%s", view)
	}
	if !strings.Contains(view, "Hello") {
		t.Errorf("the channel did not come back:\n%s", view)
	}
}

// Results for an earlier search, landing late, replaced the latest one's.
func TestModel_StaleSearchResultsAreIgnored(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	m, _ = step(t, m, palette.SearchSubmitMsg{Term: "first"})
	m, _ = step(t, m, palette.SearchSubmitMsg{Term: "second"})

	m, _ = step(t, m, tui.SearchResultsMsg{Term: "first", Posts: posts("c1", "first hit")})
	if strings.Contains(viewOf(m), "first hit") {
		t.Errorf("an earlier search's results were shown:\n%s", viewOf(m))
	}

	m, _ = step(t, m, tui.SearchResultsMsg{Term: "second", Posts: posts("c1", "second hit")})
	if !strings.Contains(viewOf(m), "second hit") {
		t.Errorf("the latest search's results were not shown:\n%s", viewOf(m))
	}
}

// User searches fire per keystroke, so their responses race each other.
func TestModel_StaleUserSearchResultsAreIgnored(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	m, _ = step(t, m, dmpicker.SearchTriggeredMsg{Term: "b"})
	m, _ = step(t, m, dmpicker.SearchTriggeredMsg{Term: "bo"})

	m, _ = step(t, m, tui.UserSearchResultsMsg{Term: "b", Users: []*model.User{{ID: "u9", Username: "barbara"}}})
	if strings.Contains(viewOf(m), "barbara") {
		t.Errorf("an earlier query's results were shown:\n%s", viewOf(m))
	}

	m, _ = step(t, m, tui.UserSearchResultsMsg{Term: "bo", Users: []*model.User{{ID: "u2", Username: "bob"}}})
	if !strings.Contains(viewOf(m), "bob") {
		t.Errorf("the latest query's results were not shown:\n%s", viewOf(m))
	}
}

// Scrolling up while a newly opened channel was still loading asked for the
// next page of the previous channel's paging state.
func TestModel_NoOlderPageWhileTheChannelLoads(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: fullPage(t, "recent", 60)}})

	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", TeamID: "t1"}})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	client.postsFetched = nil
	for range 5 {
		var cmd tea.Cmd
		m, cmd = step(t, m, tea.MouseMsg{X: 5, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
		if cmd != nil {
			if _, ok := cmd().(tea.BatchMsg); ok {
				t.Fatal("asked for an older page of a channel that has not loaded")
			}
		}
	}
}
