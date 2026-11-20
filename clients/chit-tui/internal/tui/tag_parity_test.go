package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	tagpickerpkg "github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/infobox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/prompt"
)

// Tag parity (ISSUE-022): the tag events, rename and delete from the
// picker, and the channel tags.

func ownTag() *model.Tag {
	return &model.Tag{ID: "t1", Name: "deploy", Scope: model.TagScopeChannel, ChannelID: "c1", OwnerID: "u1"}
}

// taggedModel opens c1, whose post p1 carries #deploy, and the channel
// carries #deploy too.
func taggedModel(t *testing.T) (tui.Model, *mockClient) {
	t.Helper()
	other := &model.Tag{ID: "t2", Name: "theirs", Scope: model.TagScopeChannel, ChannelID: "c1", OwnerID: "u2"}
	client := &mockClient{
		me:          &model.User{ID: "u1", Username: "alice"},
		allTags:     []*model.Tag{ownTag(), other},
		channelTags: map[string][]*model.Tag{"c1": {ownTag()}},
	}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.AllTagsLoadedMsg{Tags: client.allTags})
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, cmd := step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{
		{ID: "c1", TeamID: "t1", DisplayName: "General"}, {ID: "c2", TeamID: "t1", DisplayName: "Random"},
	}})
	m = feed(t, m, cmd)
	m, _ = step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
		{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "shipped", CreateAt: 1700000000000},
	}}})
	m, _ = step(t, m, tui.PostsTagsLoadedMsg{Tags: map[string][]*model.Tag{"p1": {ownTag()}}})
	return m, client
}

func tagEvent(name string, data map[string]any) tui.WebSocketEventMsg {
	return tui.WebSocketEventMsg{Event: model.WebSocketEvent{Event: name, Data: data}}
}

func TestTags_ChannelTagsShowOnOpen(t *testing.T) {
	m, _ := taggedModel(t)
	if !strings.Contains(viewOf(m), "╭── #deploy") {
		t.Errorf("no channel tag on the upper border:\n%s", viewOf(m))
	}
}

func TestTags_TagUpdatedRenamesEverywhere(t *testing.T) {
	m, _ := taggedModel(t)
	renamed := ownTag()
	renamed.Name = "release"
	m, _ = step(t, m, tagEvent(model.WebSocketEventTagUpdated, map[string]any{"tag": renamed}))
	v := viewOf(m)
	if strings.Contains(v, "#deploy") || strings.Count(v, "#release") < 2 {
		t.Errorf("the rename did not reach the post and the channel:\n%s", v)
	}
}

func TestTags_TagDeletedRemovesEverywhere(t *testing.T) {
	m, _ := taggedModel(t)
	m, _ = step(t, m, tagEvent(model.WebSocketEventTagDeleted, map[string]any{"tag_id": "t1"}))
	if v := viewOf(m); strings.Contains(v, "#deploy") {
		t.Errorf("a deleted tag still shows:\n%s", v)
	}
	// The tag picker no longer offers it.
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, key('t'))
	if v := viewOf(m); strings.Contains(v, "#deploy") || !strings.Contains(v, "#theirs") {
		t.Errorf("the picker after the delete:\n%s", v)
	}
}

func TestTags_ChannelTagsUpdatedEvent(t *testing.T) {
	m, _ := taggedModel(t)
	m, _ = step(t, m, tagEvent(model.WebSocketEventChannelTagsUpdated, map[string]any{
		"channel_id": "c1", "tags": []any{map[string]any{"id": "t9", "name": "urgent", "scope": "channel", "channel_id": "c1"}},
	}))
	if v := viewOf(m); !strings.Contains(v, "#urgent") {
		t.Errorf("the new channel tag does not show:\n%s", v)
	}
	m, _ = step(t, m, tagEvent(model.WebSocketEventChannelTagsUpdated, map[string]any{"channel_id": "c2", "tags": []any{}}))
	if v := viewOf(m); !strings.Contains(v, "#urgent") {
		t.Errorf("an event for another channel changed this one:\n%s", v)
	}
}

// openPicker opens the tag picker on p1 with the cursor on #deploy.
func openPicker(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, key('t'))
	if !strings.Contains(viewOf(m), "ctrl+r rename") {
		t.Fatalf("setup: tag picker not open:\n%s", viewOf(m))
	}
	return m
}

func TestTags_RenameFromThePicker(t *testing.T) {
	m, client := taggedModel(t)
	m = openPicker(t, m)
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	m, _ = step(t, m, wantMsg[tagpickerRename](t, cmd))
	if !strings.Contains(viewOf(m), "Rename #deploy") {
		t.Fatalf("no rename prompt:\n%s", viewOf(m))
	}
	// A name the server refuses is refused here, with the reason.
	m, _ = step(t, m, prompt.SubmittedMsg{Key: "rename-tag:t1", Value: "a/b"})
	if !strings.Contains(viewOf(m), "only letters") || len(client.renamedTags) != 0 {
		t.Errorf("a bad name went through: %v\n%s", client.renamedTags, viewOf(m))
	}
	// A taken name: the server's 409 shows in the prompt.
	client.err = &api.APIError{StatusCode: 409, AppError: &model.AppError{Message: "a tag with that name exists"}}
	m, cmd = step(t, m, prompt.SubmittedMsg{Key: "rename-tag:t1", Value: "taken"})
	m = feed(t, m, cmd)
	if !strings.Contains(viewOf(m), "that name is in use") {
		t.Errorf("the 409 does not show in the prompt:\n%s", viewOf(m))
	}
	client.err = nil
	m, cmd = step(t, m, prompt.SubmittedMsg{Key: "rename-tag:t1", Value: "release"})
	m = feed(t, m, cmd)
	if strings.Contains(viewOf(m), "Rename #deploy") || !strings.Contains(viewOf(m), "#release") {
		t.Errorf("the rename did not close the prompt or apply:\n%s", viewOf(m))
	}
}

// tagpickerRename is the message of Ctrl+R in the picker.
type tagpickerRename = tagpickerRenameRequest

func TestTags_OnlyTheOwnerChangesATag(t *testing.T) {
	m, client := taggedModel(t)
	m = openPicker(t, m)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	m, _ = step(t, m, wantMsg[tagpickerRename](t, cmd))
	if !strings.Contains(viewOf(m), "only the owner can change #theirs") || len(client.renamedTags) != 0 {
		t.Errorf("a rename of another's tag:\n%s", viewOf(m))
	}
}

func TestTags_DeleteFromThePickerAsksFirst(t *testing.T) {
	m, client := taggedModel(t)
	m = openPicker(t, m)
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	m, _ = step(t, m, wantMsg[tagpickerDeleteRequest](t, cmd))
	if !strings.Contains(viewOf(m), "delete #deploy and each use of it?") {
		t.Fatalf("no question:\n%s", viewOf(m))
	}
	m, cmd = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})
	m = feed(t, m, cmd)
	if strings.Join(client.deletedTags, ",") != "t1" || strings.Contains(viewOf(m), "#deploy") {
		t.Errorf("deleted = %v; view:\n%s", client.deletedTags, viewOf(m))
	}
}

func TestTags_ChtagAddRemoveFind(t *testing.T) {
	m, client := taggedModel(t)
	// add: a new name makes a channel tag, then applies it.
	m, cmd := step(t, m, input.SlashTriggerMsg{Input: "/chtag add #urgent"})
	m = feed(t, m, cmd)
	if strings.Join(client.tagsCreated, ",") != "urgent/channel/c1" || strings.Join(client.chanTagCalls, ",") != "+c1#new-tag" {
		t.Errorf("add: created %v, calls %v", client.tagsCreated, client.chanTagCalls)
	}
	// rm asks first; y removes.
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/chtag rm deploy"})
	if !strings.Contains(viewOf(m), "remove #deploy from this channel?") {
		t.Fatalf("rm did not ask:\n%s", viewOf(m))
	}
	m, cmd = step(t, m, key('y'))
	m = feed(t, m, cmd)
	if !strings.Contains(strings.Join(client.chanTagCalls, ","), "-c1#t1") {
		t.Errorf("rm calls = %v", client.chanTagCalls)
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/chtag rm nothere"})
	if !strings.Contains(viewOf(m), "#nothere is not on this channel") {
		t.Errorf("rm of a missing tag:\n%s", viewOf(m))
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/chtag"})
	if !strings.Contains(viewOf(m), "use /chtag add") {
		t.Errorf("no usage:\n%s", viewOf(m))
	}
	// find lists the channels; a digit opens one.
	client.foundChannels = []*model.TaggedChannel{{Channel: &model.Channel{ID: "c2", TeamID: "t1", DisplayName: "Random"}, Tags: []*model.Tag{ownTag()}}}
	m, cmd = step(t, m, input.SlashTriggerMsg{Input: "/chtag find deploy"})
	m = feed(t, m, cmd)
	if v := viewOf(m); !strings.Contains(v, "Channels with the tags") || !strings.Contains(v, "Random  #deploy") {
		t.Fatalf("find shows no list:\n%s", v)
	}
	m, cmd = step(t, m, key('1'))
	m, cmd = step(t, m, wantMsg[infobox.ActionMsg](t, cmd))
	m = feed(t, m, cmd)
	if v := viewOf(m); strings.Contains(v, "Channels with the tags") || !slices.Contains(client.postsFetched, "c2") {
		t.Errorf("1 did not open the channel: fetched %v\n%s", client.postsFetched, v)
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/chtag find nosuchtag"})
	if !strings.Contains(viewOf(m), "no tag #nosuchtag") {
		t.Errorf("find of an unknown tag:\n%s", viewOf(m))
	}
}

type (
	tagpickerRenameRequest = tagpickerpkg.TagRenameRequestMsg
	tagpickerDeleteRequest = tagpickerpkg.TagDeleteRequestMsg
)
