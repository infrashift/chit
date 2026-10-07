package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
)

// A DM created again, such as by starting one that exists, is not listed
// twice.
func TestModel_RecreatedDMIsListedOnce(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.UsersLoadedMsg{Users: []*model.User{{ID: "u2", Username: "bob", DisplayName: "Bobby Tables"}}})
	dm := &model.Channel{ID: "d1", Type: model.ChannelDirect, Name: "u1__u2"}
	m, _ = step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{dm}})

	m, _ = step(t, m, tui.DMCreatedMsg{Channel: dm})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlK})

	lines := strings.Split(viewOf(m), "\n")
	palette := strings.Join(lines[:len(lines)-1], "\n") // not the action bar
	if n := strings.Count(palette, "Bobby Tables"); n != 1 {
		t.Errorf("the DM is listed %d times, want once:\n%s", n, palette)
	}
}

// The mention popup offers the channel's members by name.
func TestModel_MentionPopupOffersTheChannelsMembers(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.UsersLoadedMsg{Users: []*model.User{{ID: "u2", Username: "bob"}}})
	m, _ = step(t, m, tui.ChannelMembersLoadedMsg{ChannelID: "c1", Members: []*model.ChannelMember{
		{ChannelID: "c1", UserID: "u1"}, {ChannelID: "c1", UserID: "u2"},
	}})

	m, _ = step(t, m, input.AtTriggerMsg{Prefix: "b", StartCol: 0})

	if !strings.Contains(viewOf(m), "@bob") {
		t.Errorf("bob not offered:\n%s", viewOf(m))
	}
}

// The thread inbox names a thread in a DM after the person, as the palette
// does.
func TestModel_ThreadInboxNamesDMThreads(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.UsersLoadedMsg{Users: []*model.User{{ID: "u2", Username: "bob"}}})
	m, _ = step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{{ID: "d1", Type: model.ChannelDirect, Name: "u1__u2"}}})
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/threads"})

	m, _ = step(t, m, tui.ThreadsLoadedMsg{Threads: []*model.ThreadResponse{{
		Thread: &model.Thread{PostID: "r1", ChannelID: "d1"},
		Posts:  []*model.Post{{ID: "r1", ChannelID: "d1", Content: "about the deploy"}},
	}}})

	if view := viewOf(m); !strings.Contains(view, "bob") || !strings.Contains(view, "about the deploy") {
		t.Errorf("DM thread not named for the person:\n%s", view)
	}
}

func TestModel_TagLoadErrorIsShown(t *testing.T) {
	m, _ := step(t, setupModel(t), tui.PostsTagsLoadedMsg{Err: &model.AppError{Message: "tags unavailable"}})
	if !strings.Contains(viewOf(m), "tags unavailable") {
		t.Errorf("error not shown:\n%s", viewOf(m))
	}
}

// On the sign-in screen Ctrl+C still quits, nothing loads until signed in,
// and a failed sign-in is reported on the form.
func TestModel_SignInScreen(t *testing.T) {
	m := tui.NewModel(&config.Config{}, &mockClient{}, nil, testutil.Styles(), nil, nil, nil)
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/logout"})
	if m.Init() != nil {
		t.Error("Init loads data before anyone has signed in")
	}

	m, _ = step(t, m, login.LoginErrorMsg{Err: &model.AppError{Message: "bad credentials"}})
	if !strings.Contains(viewOf(m), "bad credentials") {
		t.Errorf("sign-in error not shown:\n%s", viewOf(m))
	}

	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	wantMsg[tea.QuitMsg](t, cmd)
}
