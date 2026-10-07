package tui_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
)

func wsPosted(p map[string]any) tui.WebSocketEventMsg {
	return tui.WebSocketEventMsg{Event: model.WebSocketEvent{Event: model.WebSocketEventPosted, Data: p}}
}

// Your own reply reached the channel view from the HTTP response first, and
// the echo then returned early because the channel already had it, so the
// open thread never showed it.
func TestModel_OwnReplyAppearsInTheOpenThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)

	reply := &model.Post{ID: "r2", RootID: "p1", ChannelID: "c1", UserID: "u1", Content: "my own reply", CreateAt: 1700000002000}
	m, _ = step(t, m, tui.PostCreatedMsg{Post: reply})
	m, _ = step(t, m, wsPosted(map[string]any{
		"id": "r2", "root_id": "p1", "channel_id": "c1", "user_id": "u1",
		"message": "my own reply", "create_at": 1700000002000,
	}))

	if n := strings.Count(viewOf(m), "my own reply"); n != 1 {
		t.Errorf("reply shown %d times in the thread, want 1:\n%s", n, viewOf(m))
	}
}

// thread_updated carries the count including the new reply, and arrives
// before "posted"; adding one on "posted" as well over-counted every reply.
func TestModel_ReplyCountIsNotCountedTwice(t *testing.T) {
	m := setupModel(t)

	m, _ = step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventThreadUpdated,
		Data:  map[string]any{"thread": map[string]any{"post_id": "p1", "channel_id": "c1", "reply_count": 2}},
	}})
	m, _ = step(t, m, wsPosted(map[string]any{
		"id": "r9", "root_id": "p1", "channel_id": "c1", "user_id": "u1",
		"message": "another", "create_at": 1700000009000,
	}))

	view := viewOf(m)
	if !strings.Contains(view, "2 replies") || strings.Contains(view, "3 replies") {
		t.Errorf("want the server's count of 2:\n%s", view)
	}
}

func TestModel_DeletedReplyLeavesTheOpenThread(t *testing.T) {
	m := setupModel(t)
	m = openThread(t, m)
	if !strings.Contains(viewOf(m), "First reply") {
		t.Fatal("setup: reply not shown")
	}

	m, _ = step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{
		Event: model.WebSocketEventPostDeleted,
		Data:  map[string]any{"post_id": "r1", "channel_id": "c1"},
	}})

	if strings.Contains(viewOf(m), "First reply") {
		t.Errorf("deleted reply still shown:\n%s", viewOf(m))
	}
}

// The first channel opened on its own skipped the member fetch that opening
// one by hand does, so @-mentions offered nobody there.
func TestModel_TheFirstChannelOpensLikeAnyOther(t *testing.T) {
	client := &mockClient{}
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	_, cmd := step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})
	drain(cmd)

	if !slices.Contains(client.membersFetched, "c1") {
		t.Errorf("members fetched for %v, want c1", client.membersFetched)
	}
}

// The action bar named a DM by the other person's username while the
// palette used their display name, so the same conversation went by two
// names on one screen.
func TestModel_ActionBarNamesADMLikeThePalette(t *testing.T) {
	m := setupModel(t)
	dm := &model.Channel{ID: "d1", Type: model.ChannelDirect, Name: "u1__u2", DisplayName: "u1__u2"}
	m, _ = step(t, m, tui.DMChannelsLoadedMsg{Channels: []*model.Channel{dm}})
	m, _ = step(t, m, tui.UsersLoadedMsg{Users: []*model.User{{ID: "u2", Username: "bob", DisplayName: "Bob Smith"}}})
	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: dm})

	lines := strings.Split(viewOf(m), "\n")
	if bar := lines[len(lines)-1]; !strings.Contains(bar, "Bob Smith") {
		t.Errorf("action bar = %q, want the display name the palette shows", bar)
	}
}
