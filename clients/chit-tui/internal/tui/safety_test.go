package tui_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

var ownPost = &model.Post{ID: "mine", ChannelID: "c1", UserID: "u1", Content: "my old words", CreateAt: 1700000000000}

// withOwnPost is a session with one of the user's own posts selected in a
// focused history pane, where single-letter keys act on it.
func withOwnPost(t *testing.T) (tui.Model, *mockClient) {
	t.Helper()
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", TeamID: "t1", DisplayName: "General"},
		{ID: "c2", TeamID: "t1", DisplayName: "Random"},
	}})
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{ownPost}}})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != tui.FocusViewport {
		t.Fatalf("setup: focus = %v, want the history pane", m.Focused())
	}
	return m, client
}

func send(t *testing.T, m tui.Model, text string) tui.Model {
	t.Helper()
	m, cmd := step(t, m, input.SendMsg{Content: text})
	drain(cmd)
	return m
}

// An edit started and forgotten used to survive a channel switch, so the
// next message sent anywhere overwrote the old post.
func TestModel_SwitchingChannelAbandonsAnEdit(t *testing.T) {
	m, client := withOwnPost(t)
	m, _ = step(t, m, key('e'))
	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", TeamID: "t1"}})

	_ = send(t, m, "hello random")

	if len(client.editedPosts) != 0 {
		t.Errorf("the message overwrote post %v", client.editedPosts)
	}
	if client.lastCreatedPost == nil || client.lastCreatedPost.ChannelID != "c2" {
		t.Errorf("want a new post in c2, got %+v", client.lastCreatedPost)
	}
}

func TestModel_EscAbandonsAnEdit(t *testing.T) {
	m, client := withOwnPost(t)
	m, _ = step(t, m, key('e'))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	// Once in the history; a second copy would be the input.
	if strings.Count(viewOf(m), "my old words") > 1 {
		t.Errorf("the abandoned edit was left in the input:\n%s", viewOf(m))
	}
	_ = send(t, m, "something new")
	if len(client.editedPosts) != 0 {
		t.Errorf("the message overwrote post %v", client.editedPosts)
	}
}

// A reply goes through its own branch, so the edit used to wait behind it
// for the next channel message.
func TestModel_OpeningAThreadAbandonsAnEdit(t *testing.T) {
	m, client := withOwnPost(t)
	m, _ = step(t, m, key('e'))
	m, _ = step(t, m, viewport.PostSelectedMsg{Post: ownPost})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	_ = send(t, m, "after the thread")
	if len(client.editedPosts) != 0 {
		t.Errorf("the message overwrote post %v", client.editedPosts)
	}
}

func TestModel_EditingIsShown(t *testing.T) {
	m, _ := withOwnPost(t)
	m, _ = step(t, m, key('e'))

	if !strings.Contains(viewOf(m), "editing") {
		t.Errorf("nothing says a post is being edited:\n%s", viewOf(m))
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if strings.Contains(viewOf(m), "editing") {
		t.Errorf("still says editing after Esc:\n%s", viewOf(m))
	}
}

// A stray "d" deleted the selected post outright.
func TestModel_DeleteAsksFirst(t *testing.T) {
	m, client := withOwnPost(t)

	m, cmd := step(t, m, key('d'))
	drain(cmd)
	if len(client.deletedPosts) != 0 {
		t.Fatal("deleted without asking")
	}
	if !strings.Contains(viewOf(m), "delete this message?") {
		t.Errorf("no confirmation asked:\n%s", viewOf(m))
	}

	_, cmd = step(t, m, key('d'))
	drain(cmd)
	if !slices.Equal(client.deletedPosts, []string{"mine"}) {
		t.Errorf("deleted = %v, want [mine] after confirming", client.deletedPosts)
	}
}

func TestModel_AnyOtherKeyCancelsADelete(t *testing.T) {
	m, client := withOwnPost(t)

	m, _ = step(t, m, key('d'))
	m, _ = step(t, m, key('k'))
	if strings.Contains(viewOf(m), "delete this message?") {
		t.Errorf("the question outlived being canceled:\n%s", viewOf(m))
	}
	m, cmd := step(t, m, key('d'))
	drain(cmd)
	if len(client.deletedPosts) != 0 {
		t.Error("a canceled delete went through on the next d")
	}
	if !strings.Contains(viewOf(m), "delete this message?") {
		t.Errorf("the next d should ask again:\n%s", viewOf(m))
	}
}

// The tag picker opened from a thread handed focus to the hidden history
// pane on close, where a following "d" acted on a post the reader could not
// see.
func TestModel_TagPickerFromAThreadReturnsToTheThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != tui.FocusThread {
		t.Fatalf("setup: focus = %v, want the thread", m.Focused())
	}

	m, _ = step(t, m, key('t'))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	if m.Focused() != tui.FocusThread {
		t.Errorf("focus = %v after closing the tag picker, want the thread", m.Focused())
	}
}

// When someone deleted the root of the thread being replied to, focus jumped
// to the history pane and the rest of the reply was read as commands.
func TestModel_RootDeletedWhileTypingKeepsTheInput(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)
	if m.Focused() != tui.FocusInput {
		t.Fatalf("setup: focus = %v, want the input", m.Focused())
	}

	m, _ = step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostDeleted,
		Data:  map[string]any{"post_id": "p1", "channel_id": "c1"},
	}})

	if m.Focused() != tui.FocusInput {
		t.Errorf("focus = %v, want it left on the input", m.Focused())
	}
}

// Picking a person with the mouse closed the palette but left focus on it,
// so the keyboard did nothing until Tab.
func TestModel_MousePickInThePaletteRestoresFocus(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	m, _ = step(t, m, tui.UserSearchResultsMsg{Users: []*model.User{{ID: "u2", Username: "bob"}}})
	if !strings.Contains(viewOf(m), "bob") {
		t.Fatalf("setup: bob not listed:\n%s", viewOf(m))
	}

	m, _ = step(t, m, tea.MouseMsg{X: 60, Y: 6, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})

	if m.Focused() == tui.FocusPalette {
		t.Error("focus stayed on the closed palette")
	}
}

// The question can time out or be pushed off the status line by something
// else. A "d" after that must ask again, not act on a question no longer
// shown.
func TestModel_DeleteIsOnlyConfirmedWhileTheQuestionShows(t *testing.T) {
	m, client := withOwnPost(t)

	m, _ = step(t, m, key('d'))
	m, _ = step(t, m, tui.ChannelViewedMsg{ChannelID: "c1", Err: errors.New("unrelated")})
	_, cmd := step(t, m, key('d'))
	drain(cmd)

	if len(client.deletedPosts) != 0 {
		t.Error("deleted on a question no longer shown")
	}
}

// messagesOf runs a command and returns its messages, unwrapping batches.
func messagesOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
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

func createdPost(t *testing.T, cmd tea.Cmd) tui.PostCreatedMsg {
	t.Helper()
	for _, msg := range messagesOf(cmd) {
		if created, ok := msg.(tui.PostCreatedMsg); ok {
			return created
		}
	}
	t.Fatal("no PostCreatedMsg")
	return tui.PostCreatedMsg{}
}

// Tags waited in one slot on the model for the next post to be created, so
// two quick sends gave the first post the second one's tags.
func TestModel_TagsGoToThePostThatCarriedThem(t *testing.T) {
	m, client := withOwnPost(t)
	m, _ = step(t, m, tui.AllTagsLoadedMsg{Tags: []*model.Tag{{ID: "ta", Name: "alpha"}, {ID: "tb", Name: "beta"}}})

	client.post = &model.Post{ID: "one", ChannelID: "c1", UserID: "u1"}
	m, cmd := step(t, m, input.SendMsg{Content: "first #alpha"})
	first := createdPost(t, cmd)
	client.post = &model.Post{ID: "two", ChannelID: "c1", UserID: "u1"}
	m, cmd = step(t, m, input.SendMsg{Content: "second #beta"})
	second := createdPost(t, cmd)

	// The second send's response lands first.
	m, cmd = step(t, m, second)
	drain(cmd)
	_, cmd = step(t, m, first)
	drain(cmd)

	slices.Sort(client.taggedPosts)
	if want := []string{"one#ta", "two#tb"}; !slices.Equal(client.taggedPosts, want) {
		t.Errorf("tagged = %v, want %v", client.taggedPosts, want)
	}
}

// Editing stripped "#prod" from the text and then dropped the tag, so the
// word vanished and nothing was tagged.
func TestModel_EditAppliesItsTags(t *testing.T) {
	m, client := withOwnPost(t)
	m, _ = step(t, m, tui.AllTagsLoadedMsg{Tags: []*model.Tag{{ID: "tp", Name: "prod"}}})
	m, _ = step(t, m, key('e'))

	_ = send(t, m, "deployed #prod")

	if !slices.Equal(client.editedPosts, []string{"mine"}) {
		t.Fatalf("edited = %v, want [mine]", client.editedPosts)
	}
	if !slices.Equal(client.taggedPosts, []string{"mine#tp"}) {
		t.Errorf("tagged = %v, want [mine#tp]", client.taggedPosts)
	}
}
