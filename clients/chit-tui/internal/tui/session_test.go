package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

var (
	alice = &model.User{ID: "u1", Username: "alice"}
	bob   = &model.User{ID: "u2", Username: "bob"}
)

const aliceHistory = "something only alice should see"

// signedInAsAlice is a running session with a channel open and a post in it.
func signedInAsAlice(t *testing.T) (tui.Model, *mockClient, *mockWSClient) {
	t.Helper()
	client := &mockClient{me: alice}
	wsc := newMockWSClient()
	m := tui.NewModel(&config.Config{ServerURL: "http://localhost:8065"}, client, wsc,
		styles.New(theme.TokyoNight()), nil, nil, nil)

	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 120, Height: 40},
		tui.UserLoadedMsg{User: alice},
		tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1", DisplayName: "Engineering"}}},
		tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1", DisplayName: "Secret Plans"}}},
		tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", ChannelID: "c1", UserID: "u1", Content: aliceHistory, CreateAt: 1700000000000},
		}}},
		tui.UsersLoadedMsg{Users: []*model.User{alice}},
	} {
		updated, _ := m.Update(msg)
		m = updated.(tui.Model)
	}
	if !strings.Contains(testutil.StripANSI(m.View()), aliceHistory) {
		t.Fatal("setup: alice's history is not on screen")
	}
	return m, client, wsc
}

func step(t *testing.T, m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

func signOut(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/logout"})
	if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
		t.Fatal("not signed out")
	}
	return m
}

func expire(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	m, _ = step(t, m, tui.ThreadLoadedMsg{Err: &api.APIError{StatusCode: 401}})
	if !strings.Contains(testutil.StripANSI(m.View()), "Chit Login") {
		t.Fatal("expiry did not prompt sign-in")
	}
	return m
}

// Signing out reset nothing, so whoever signed in next was shown the previous
// user's channel and history until they navigated away.
func TestModel_SignOutForgetsTheSession(t *testing.T) {
	m, _, _ := signedInAsAlice(t)

	m = signOut(t, m)
	m, _ = step(t, m, login.LoginSuccessMsg{Token: "bob-token"})
	m, _ = step(t, m, tui.UserLoadedMsg{User: bob})

	view := testutil.StripANSI(m.View())
	for _, leaked := range []string{aliceHistory, "Secret Plans"} {
		if strings.Contains(view, leaked) {
			t.Errorf("bob's session shows alice's %q:\n%s", leaked, view)
		}
	}
}

func TestModel_SignOutTwiceClosesTheSocketEachTime(t *testing.T) {
	m, _, wsc := signedInAsAlice(t)

	m = signOut(t, m)
	m, _ = step(t, m, login.LoginSuccessMsg{Token: "again"})
	_ = signOut(t, m)

	if wsc.closes != 2 {
		t.Errorf("closes = %d, want 2", wsc.closes)
	}
}

// Messages posted while the session was expired never arrive over the
// socket, so signing back in has to re-read the open channel.
func TestModel_ReLoginAfterExpiryReloadsTheChannel(t *testing.T) {
	m, client, _ := signedInAsAlice(t)

	m = expire(t, m)
	m, _ = step(t, m, login.LoginSuccessMsg{Token: "renewed"})
	client.postsFetched = nil
	m, cmd := step(t, m, tui.UserLoadedMsg{User: alice})
	drain(cmd)

	if !slices.Contains(client.postsFetched, "c1") {
		t.Errorf("open channel not reloaded after re-login; fetched %v", client.postsFetched)
	}
	if !strings.Contains(testutil.StripANSI(m.View()), "Secret Plans") {
		t.Error("the same user lost their open channel")
	}
}

// The re-login screen takes any credentials. Someone else signing in there
// must not inherit the expired session.
func TestModel_ReLoginAsSomeoneElseForgetsTheSession(t *testing.T) {
	m, _, _ := signedInAsAlice(t)

	m = expire(t, m)
	m, _ = step(t, m, login.LoginSuccessMsg{Token: "bob-token"})
	m, _ = step(t, m, tui.UserLoadedMsg{User: bob})

	if view := testutil.StripANSI(m.View()); strings.Contains(view, aliceHistory) {
		t.Errorf("bob's session shows alice's history:\n%s", view)
	}
}

func commandResponse(slug, text string) tui.WebSocketEventMsg {
	return tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventCommandResponse,
		Data:  map[string]any{"text": text, "channel_id": "c1", "command_slug": slug},
	}}
}

// Authors still being looked up are nil placeholders in the user map. Naming
// the command's pseudo-author read every entry, so a post by a user the
// server had not returned crashed the client on the next command.
func TestModel_CommandResponseWithAnUnresolvedAuthor(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
		{ID: "p2", ChannelID: "c1", UserID: "ghost", Content: "from a deleted user", CreateAt: 1700000000000},
	}}})

	m, _ = step(t, m, commandResponse("help", "the help text"))

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "the help text") || !strings.Contains(view, "/help") {
		t.Errorf("command output missing:\n%s", view)
	}
}

// Every response to a command shared one post ID, and the render cache is
// keyed by ID, so a second /help showed the first one's text again.
func TestModel_RepeatedCommandShowsItsNewOutput(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, commandResponse("echo", "first output"))
	m, _ = step(t, m, commandResponse("echo", "second output"))

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "first output") || !strings.Contains(view, "second output") {
		t.Errorf("want both outputs shown:\n%s", view)
	}
}

// Loading users rebuilt the name map from real users alone, so command output
// lost its "/slug" author and showed the raw pseudo-ID.
func TestModel_CommandAuthorSurvivesAUserLoad(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, commandResponse("help", "the help text"))
	m, _ = step(t, m, tui.UsersLoadedMsg{Users: []*model.User{bob}})

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "/help") {
		t.Errorf("command author lost after a user load:\n%s", view)
	}
	if strings.Contains(view, "chit:") {
		t.Errorf("raw pseudo-ID shown:\n%s", view)
	}
}

// Each command has its own author, so earlier output keeps its name.
func TestModel_EachCommandKeepsItsOwnAuthor(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, commandResponse("help", "the help text"))
	m, _ = step(t, m, commandResponse("echo", "echoed"))

	view := testutil.StripANSI(m.View())
	if !strings.Contains(view, "/help") || !strings.Contains(view, "/echo") {
		t.Errorf("want both command authors shown:\n%s", view)
	}
}
