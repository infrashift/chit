package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
	"github.com/muesli/termenv"
)

// Pinning showed only when the WebSocket echo arrived. With the socket down
// the post stayed unpinned on screen, and a second p pinned it again.
func TestModel_PinShowsWithoutTheEcho(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.PostPinnedMsg{PostID: "p1", Pinned: true})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	_, cmd := step(t, m, key('p'))

	if got := wantMsg[tui.PostPinnedMsg](t, cmd); got.Pinned {
		t.Error("p on a pinned post pinned it again, want unpin")
	}
}

// With no channel open, or before the user has loaded, a sent message went
// nowhere and the input cleared as if it had been sent.
func TestModel_SendWithNoChannelSaysSo(t *testing.T) {
	m := modelWithClient(t, &mockClient{})

	m, cmd := step(t, m, input.SendMsg{Content: "into the void"})

	for _, msg := range messagesOf(cmd) {
		if _, ok := msg.(tui.PostCreatedMsg); ok {
			t.Fatal("sent with no channel to send to")
		}
	}
	if !strings.Contains(viewOf(m), "not sent") {
		t.Errorf("nothing says the message was not sent:\n%s", viewOf(m))
	}
}

// A search hit in another channel opened that channel but left the history
// wherever it landed, so the hit itself still had to be found by hand.
func TestModel_SearchHitElsewhereIsSelectedOnceLoaded(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", TeamID: "t1", DisplayName: "General"},
		{ID: "c2", TeamID: "t1", DisplayName: "Random"},
	}})

	m, _ = step(t, m, palette.PostChosenMsg{Post: &model.Post{ID: "c2-the hit", ChannelID: "c2"}})
	// Newest first: the hit is not the post selected by default.
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c2", Posts: posts("c2", "later", "the hit", "earlier")})

	if got := m.SelectedPostID(); got != "c2-the hit" {
		t.Errorf("selected %q, want the search hit", got)
	}
}

// The wheel over an open thread scrolls the thread, but it also paged the
// channel history hidden behind it whenever that was at the top.
func TestModel_WheelOverAThreadDoesNotPageTheChannel(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	// Tall enough that a full page fits, so the history starts at the top.
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 600})
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})
	page := make([]string, 60)
	for i := range page {
		page[i] = fmt.Sprintf("m%d", i)
	}
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: posts("c1", page...)})
	wheelUp := tea.MouseMsg{X: 5, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}

	m, _ = step(t, m, viewport.PostSelectedMsg{Post: &model.Post{ID: "c1-m0", ChannelID: "c1"}})
	client.postsFetched = nil
	_, cmd := step(t, m, wheelUp)
	drain(cmd)

	if len(client.postsFetched) != 0 {
		t.Errorf("paged the hidden channel history: fetched %v", client.postsFetched)
	}
}

// A theme picked during the session was not applied to the sign-in screen,
// which then showed the old theme on the next sign-in.
func TestModel_ThemeChangeReachesTheSignInScreen(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	signIn := func(pick bool) string {
		m := setupModel(t)
		if pick {
			m, _ = step(t, m, skinpicker.SkinSelectedMsg{Name: "catppuccin-latte"})
		}
		m, _ = step(t, m, tui.ThreadLoadedMsg{Err: &api.APIError{StatusCode: 401}})
		return m.View()
	}

	if signIn(true) == signIn(false) {
		t.Error("the sign-in screen looks the same after a theme change")
	}
}
