package tui_test

import (
	"bytes"
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
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })
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
		if strings.Contains(lastFrame(tm), want) {
			break
		}
	}
	return finalView(t, tm)
}

type probeMsg struct{}

func lastFrame(tm *teatest.TestModel) string {
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(tm.Output())
	return testutil.StripANSI(buf.String())
}
