package tui_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
)

// feed runs cmd and passes every message it produces back into the model,
// as Bubble Tea would, one level deep.
func feed(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	for _, msg := range messagesOf(cmd) {
		m, _ = step(t, m, msg)
	}
	return m
}

// A user the server does not return, such as a deleted one, stayed a nil
// placeholder and was asked for again on every post, page and DM load.
func TestModel_UsersTheServerDoesNotReturnAreNotAskedForAgain(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})

	ghostPost := func(id string) *model.PostList {
		return &model.PostList{Order: []*model.Post{{ID: id, ChannelID: "c1", UserID: "ghost", Content: "boo", CreateAt: 1700000000000}}}
	}
	m, cmd := step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: ghostPost("g1")})
	m = feed(t, m, cmd)
	if !slices.Contains(client.usersFetched, "ghost") {
		t.Fatalf("setup: ghost never looked up; fetched %v", client.usersFetched)
	}

	client.usersFetched = nil
	_, cmd = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: ghostPost("g2")})
	drain(cmd)
	if slices.Contains(client.usersFetched, "ghost") {
		t.Error("asked again for a user the server already did not return")
	}
}

// A failed lookup is worth retrying, unlike one the server answered.
func TestModel_UsersAreAskedForAgainAfterAFailedLookup(t *testing.T) {
	m := setupModel(t)
	ghostPost := func(id string) *model.PostList {
		return &model.PostList{Order: []*model.Post{{ID: id, ChannelID: "c1", UserID: "ghost", Content: "boo", CreateAt: 1700000000000}}}
	}
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: ghostPost("g1")})
	m, _ = step(t, m, tui.UsersLoadedMsg{Err: errors.New("boom"), Requested: []string{"ghost"}})
	_, cmd := step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: ghostPost("g2")})

	found := false
	for _, msg := range messagesOf(cmd) {
		if loaded, ok := msg.(tui.UsersLoadedMsg); ok && slices.Contains(loaded.Requested, "ghost") {
			found = true
		}
	}
	if !found {
		t.Error("ghost was not looked up again")
	}
}

// Every DM event reloads the DM list, which then fetched member rows for every
// DM again: one request per conversation, each time.
func TestModel_DMListReloadFetchesMembersOnlyForNewDMs(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	d1 := &model.Channel{ID: "d1", Type: model.ChannelDirect, Name: "u1__u2"}
	d2 := &model.Channel{ID: "d2", Type: model.ChannelDirect, Name: "u1__u3"}

	m, cmd := step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{d1}})
	m = feed(t, m, cmd)
	client.membersFetched = nil

	_, cmd = step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{d1, d2}})
	drain(cmd)

	if !slices.Equal(client.membersFetched, []string{"d2"}) {
		t.Errorf("members fetched for %v, want only the new d2", client.membersFetched)
	}
}

// Tagging a post refetched the whole tag list each time, though only a newly
// created tag adds anything to it.
func TestModel_TaggingWithAKnownTagDoesNotReloadTheTagList(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)

	_, cmd := step(t, m, tui.TagAddedToPostMsg{PostID: "p1", TagID: "t1"})
	drain(cmd)

	if client.allTagsFetches != 0 {
		t.Errorf("tag list fetched %d times, want 0", client.allTagsFetches)
	}
}

// A tag created from the open picker with Ctrl+N was applied, but the picker
// went on listing the tags it opened with, so the new one never appeared.
func TestModel_TagCreatedInThePickerAppearsInIt(t *testing.T) {
	m := setupModel(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	for _, r := range "fresh" {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Ctrl+N asks the app to create the tag; the app's command creates and
	// applies it, and its result comes back to the app.
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	for _, msg := range messagesOf(cmd) {
		var next tea.Cmd
		m, next = step(t, m, msg)
		m = feed(t, m, next)
	}

	if !strings.Contains(viewOf(m), "[x] #fresh") {
		t.Errorf("the new tag is not listed as applied:\n%s", viewOf(m))
	}
}
