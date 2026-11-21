package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
)

// The TUI shows the HALO data: the kind of an author, the client that
// posted for a person, the provenance, the files and the streams.

func haloClient() *mockClient {
	return &mockClient{
		me: &model.User{ID: "u1", Username: "alice"},
		users: []*model.User{
			{ID: "agent1", Username: "chit-agent", ActorType: model.ActorTypeAgent},
			{ID: "halod", Username: "halod", ActorType: model.ActorTypeAgent},
		},
		files: map[string]*model.File{
			"file-1": {ID: "file-1", Name: "chart.png", ContentType: "image/png", Width: 4, Height: 3, SizeBytes: 2048},
		},
	}
}

// haloModel loads one channel whose history holds an agent post with
// provenance and a file, and a post that halod made for alice.
func haloModel(t *testing.T, client *mockClient) (tui.Model, tea.Cmd) {
	t.Helper()
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1", DisplayName: "General"}, {ID: "c2", TeamID: "t1", DisplayName: "Random"}}})
	return step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
		{ID: "p2", ChannelID: "c1", UserID: "u1", Content: "drafted for me", CreateAt: 1700000001000,
			Props: map[string]any{model.PropOnBehalfOf: map[string]any{"actor_id": "halod"}}},
		{ID: "p1", ChannelID: "c1", UserID: "agent1", Content: "the chart", FileIDs: []string{"file-1"}, CreateAt: 1700000000000,
			Props: map[string]any{model.PropProvenance: map[string]any{"model": "claude-opus-5-5", "tool_ids": []any{"tool-a", "tool-b"}}}},
	}}})
}

func TestHALO_PostsShowKindViaProvenanceAndFiles(t *testing.T) {
	client := haloClient()
	m, cmd := haloModel(t, client)
	m = feed(t, m, cmd)
	v := viewOf(m)
	for _, want := range []string{"chit-agent [agent]", "[provenance]", "[via @halod]", "[file] chart.png  image/png  4×3  2.0 KB"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	if strings.Join(client.fileInfoCalls, ",") != "file-1" {
		t.Errorf("file info calls = %v, want [file-1]", client.fileInfoCalls)
	}
}

func TestHALO_FileInfosComeInBatches(t *testing.T) {
	client := haloClient()
	m := modelWithClient(t, client)
	m, _ = step(t, m, tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}})
	m, _ = step(t, m, tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}}})
	var posts []*model.Post
	for i := range 50 {
		posts = append(posts, &model.Post{ID: fmt.Sprintf("p%d", i), ChannelID: "c1", UserID: "u1", FileIDs: []string{fmt.Sprintf("f%02d", i)}})
	}
	m, cmd := step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: posts}})
	first := wantMsg[tui.FileInfosLoadedMsg](t, cmd)
	if len(client.fileInfoCalls) != 40 {
		t.Fatalf("first batch asked for %d files, want 40", len(client.fileInfoCalls))
	}
	m, cmd = step(t, m, first)
	wantMsg[tui.FileInfosLoadedMsg](t, cmd)
	if len(client.fileInfoCalls) != 50 {
		t.Errorf("after two batches: %d calls, want 50", len(client.fileInfoCalls))
	}
	if v := viewOf(m); !strings.Contains(v, "[file f00]") {
		t.Errorf("no placeholder for a file with no record:\n%s", v)
	}
}

// A 429 stops the batch; the rest waits for Retry-After, then goes on.
func TestHALO_FileInfosWaitOutARateLimit(t *testing.T) {
	client := haloClient()
	client.err = &api.APIError{StatusCode: 429, RetryAfter: 2 * time.Second}
	m, cmd := haloModel(t, client)
	msg := wantMsg[tui.FileInfosLoadedMsg](t, cmd)
	if len(msg.Left) != 1 || msg.RetryAfter != 2*time.Second {
		t.Fatalf("msg = %+v, want file-1 left and a 2s wait", msg)
	}
	client.err = nil
	m, cmd = step(t, m, msg)
	if got := messagesOf(cmd); len(got) != 0 {
		t.Errorf("the client asked again before the wait: %v", got)
	}
	if v := viewOf(m); !strings.Contains(v, "[file file-1]") {
		t.Errorf("no placeholder while the record waits:\n%s", v)
	}
}

func streamEvent(channelID, userID, status, streamID string) tui.WebSocketEventMsg {
	return tui.WebSocketEventMsg{Event: model.WebSocketEvent{Event: model.WebSocketEventStreamStatus, Data: map[string]any{
		"channel_id": channelID, "user_id": userID, "status": status, "stream_id": streamID,
	}}}
}

func TestHALO_StreamIndicator(t *testing.T) {
	m, cmd := haloModel(t, haloClient())
	m = feed(t, m, cmd)
	m, _ = step(t, m, streamEvent("c2", "agent1", model.StreamStarted, "s1"))
	if strings.Contains(viewOf(m), "is responding") {
		t.Fatal("a stream in another channel shows")
	}
	m, _ = step(t, m, streamEvent("c1", "agent1", model.StreamStarted, "s1"))
	if !strings.Contains(viewOf(m), "chit-agent is responding…") {
		t.Fatalf("no indicator:\n%s", viewOf(m))
	}
	m, _ = step(t, m, streamEvent("c1", "agent1", model.StreamFinished, "s1"))
	if strings.Contains(viewOf(m), "is responding") {
		t.Error("finished left the indicator")
	}
	// The reply of the agent ends its stream too.
	m, _ = step(t, m, streamEvent("c1", "agent1", model.StreamProgress, "s2"))
	m, _ = step(t, m, tui.WebSocketEventMsg{Event: model.WebSocketEvent{Event: model.WebSocketEventPosted,
		Data: map[string]any{"id": "p9", "channel_id": "c1", "user_id": "agent1", "content": "done"}}})
	if strings.Contains(viewOf(m), "is responding") {
		t.Error("the reply left the indicator")
	}
	// A switch of channel clears the indicators.
	m, _ = step(t, m, streamEvent("c1", "agent1", model.StreamStarted, "s3"))
	if !strings.Contains(viewOf(m), "is responding") {
		t.Fatal("setup: no indicator before the switch")
	}
	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", TeamID: "t1", DisplayName: "Random"}})
	if strings.Contains(viewOf(m), "is responding") {
		t.Error("the indicator followed the user to another channel")
	}
}

func TestHALO_PostDetails(t *testing.T) {
	m, cmd := haloModel(t, haloClient())
	m = feed(t, m, cmd)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, key('k'))
	m, _ = step(t, m, key('i'))
	v := viewOf(m)
	for _, want := range []string{"Post details", "@chit-agent (agent)", "claude-opus-5-5", "tool-a", "tool-b", "[file] chart.png"} {
		if !strings.Contains(v, want) {
			t.Errorf("details lack %q:\n%s", want, v)
		}
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if strings.Contains(viewOf(m), "Post details") {
		t.Error("Esc left the details open")
	}
	m, _ = step(t, m, key('j'))
	m, _ = step(t, m, key('i'))
	if v := viewOf(m); !strings.Contains(v, "@halod on behalf of the author") {
		t.Errorf("details of the on-behalf-of post lack the actor:\n%s", v)
	}
}

func TestHALO_PeopleListsMarkMachines(t *testing.T) {
	client := haloClient()
	client.searchUsers = []*model.User{{ID: "agent1", Username: "chit-agent", ActorType: model.ActorTypeAgent}}
	m, cmd := haloModel(t, client)
	m = feed(t, m, cmd)

	people, _ := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	people, cmd = step(t, people, palette.UserQueryMsg{Term: "chit"})
	people = feed(t, people, cmd)
	if v := viewOf(people); !strings.Contains(v, "@chit-agent [agent]") {
		t.Errorf("the palette does not mark the agent:\n%s", v)
	}

	group, _ := step(t, m, input.SlashTriggerMsg{Input: "/group"})
	group, cmd = step(t, group, dmpicker.SearchTriggeredMsg{Term: "chit"})
	group = feed(t, group, cmd)
	if v := viewOf(group); !strings.Contains(v, "@chit-agent [agent]") {
		t.Errorf("the member picker does not mark the agent:\n%s", v)
	}
}
