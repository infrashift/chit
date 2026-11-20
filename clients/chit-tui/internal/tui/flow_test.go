package tui_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
)

// These run the model in a real Bubble Tea program, so commands execute and
// their messages come back the way they do in the client, rather than being
// fed in by hand.

func runningProgram(t *testing.T) *teatest.TestModel {
	t.Helper()
	client := &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", TeamID: "t1", DisplayName: "General"}},
		posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "welcome aboard", CreateAt: 1700000000000},
		}},
		users: []*model.User{{ID: "u1", Username: "alice"}},
	}
	m := tui.NewModel(&config.Config{ServerURL: "http://localhost:8065"}, client, nil,
		testutil.Styles(), nil, nil, nil)
	m.SetClipboardOutput(io.Discard)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Errorf("quit: %v", err)
		}
	})
	return tm
}

func waitForScreen(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains([]byte(testutil.StripANSI(string(out))), []byte(want))
	}, teatest.WithDuration(5*time.Second))
}

func finalView(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	return testutil.StripANSI(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View())
}

// Starting up loads the user, their teams and channels, opens the first
// channel and shows its history; a message typed and sent appears in it.
func TestFlow_StartUpAndSendAMessage(t *testing.T) {
	tm := runningProgram(t)
	waitForScreen(t, tm, "welcome aboard")

	tm.Type("hello team")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// The placeholder returns once the input has been cleared by sending.
	waitForScreen(t, tm, "Type a message")

	view := finalView(t, tm)
	if !strings.Contains(view, "Engineering > General") {
		t.Errorf("the first channel was not opened:\n%s", view)
	}
	if n := strings.Count(view, "hello team"); n != 1 {
		t.Errorf("sent message shown %d times, want once in the history and not left in the input:\n%s", n, view)
	}
	// Your own posts are signed with your name, not your user ID.
	if strings.Contains(view, "│ u1 ") {
		t.Errorf("own post signed with the raw user ID:\n%s", view)
	}
}

// An expired session asks for sign-in; signing back in reloads the session
// and returns to the channel.
func TestFlow_SessionExpiresAndSignsBackIn(t *testing.T) {
	tm := runningProgram(t)
	waitForScreen(t, tm, "welcome aboard")

	tm.Send(tui.ChannelViewedMsg{ChannelID: "c1", Err: &api.APIError{StatusCode: 401}})
	waitForScreen(t, tm, "Chit Login")

	tm.Send(login.LoginSuccessMsg{Token: "renewed"})

	view := finalViewAfter(t, tm, "welcome aboard")
	if strings.Contains(view, "Chit Login") {
		t.Errorf("still on the sign-in screen:\n%s", view)
	}
}

// finalViewAfter waits until want is in the final model's view, since the
// output stream already holds it from before the expiry.
func finalViewAfter(t *testing.T, tm *teatest.TestModel, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		tm.Send(probeMsg{})
		if strings.Contains(lastFrame(t, tm), want) {
			break
		}
	}
	return finalView(t, tm)
}

type probeMsg struct{}

func lastFrame(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tm.Output()); err != nil {
		t.Fatalf("read output: %v", err)
	}
	return testutil.StripANSI(buf.String())
}

// programWith runs the model with client in a real Bubble Tea program.
func programWith(t *testing.T, client *mockClient) *teatest.TestModel {
	t.Helper()
	m := tui.NewModel(&config.Config{ServerURL: "http://localhost:8065"}, client, nil,
		testutil.Styles(), nil, nil, nil)
	m.SetClipboardOutput(io.Discard)
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
}

func flowClient() *mockClient {
	return &mockClient{
		me:       &model.User{ID: "u1", Username: "alice"},
		teams:    []*model.Team{{ID: "t1", DisplayName: "Engineering"}},
		channels: []*model.Channel{{ID: "c1", TeamID: "t1", DisplayName: "General"}},
		posts: &model.PostList{Order: []*model.Post{
			{ID: "p1", ChannelID: "c1", UserID: "u1", Content: "welcome aboard", CreateAt: 1700000000000},
		}},
		users: []*model.User{{ID: "u1", Username: "alice"}},
	}
}

// The delete question (ui/confirm) shows in the action bar, and the second
// d deletes the post.
func TestFlow_DeleteAsksThenDeletes(t *testing.T) {
	client := flowClient()
	tm := programWith(t, client)
	waitForScreen(t, tm, "welcome aboard")

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	waitForScreen(t, tm, "delete this message?")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	finalViewAfter(t, tm, "welcome aboard")
	if strings.Join(client.deletedPosts, ",") != "p1" {
		t.Errorf("deleted = %v, want [p1]", client.deletedPosts)
	}
}

// /group picks people as chips (ui/chips); Backspace in the empty search
// box removes the last pick.
func TestFlow_GroupPickerChips(t *testing.T) {
	client := flowClient()
	client.searchUsers = []*model.User{{ID: "u2", Username: "bob", DisplayName: "Bob"}}
	tm := programWith(t, client)
	waitForScreen(t, tm, "welcome aboard")

	tm.Type("/group")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForScreen(t, tm, "Search users")
	tm.Type("bo")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForScreen(t, tm, "[ ] @bob (Bob)")
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	waitForScreen(t, tm, "[x] @bob (Bob)")
	tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})

	view := finalViewAfter(t, tm, "[ ] @bob (Bob)")
	if !strings.Contains(view, "[ ] @bob (Bob)") {
		t.Errorf("Backspace did not remove the pick:\n%s", view)
	}
}

// Ctrl+T opens the theme picker in a real program; a move previews the next
// theme and Esc puts the first one back.
func TestFlow_ThemePreviewAndUndo(t *testing.T) {
	tm := programWith(t, flowClient())
	waitForScreen(t, tm, "welcome aboard")

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlT})
	waitForScreen(t, tm, "Esc undo")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})

	view := finalViewAfter(t, tm, "Type a message")
	if strings.Contains(view, "Select Theme") {
		t.Errorf("Esc left the theme picker open:\n%s", view)
	}
}
