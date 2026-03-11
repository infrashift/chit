package testutil_test

import (
	"testing"

	"github.com/infrashift/chit-tui/internal/testutil"
)

func TestNewTestUser(t *testing.T) {
	u := testutil.NewTestUser()
	if u.ID == "" || u.Username == "" || u.Email == "" {
		t.Error("NewTestUser returned empty required fields")
	}
}

func TestNewTestTeam(t *testing.T) {
	tm := testutil.NewTestTeam()
	if tm.ID == "" || tm.Name == "" || tm.Type == "" {
		t.Error("NewTestTeam returned empty required fields")
	}
}

func TestNewTestChannel(t *testing.T) {
	ch := testutil.NewTestChannel()
	if ch.ID == "" || ch.Name == "" || ch.Type == "" || ch.TeamID == "" {
		t.Error("NewTestChannel returned empty required fields")
	}
}

func TestNewTestPost(t *testing.T) {
	p := testutil.NewTestPost()
	if p.ID == "" || p.ChannelID == "" || p.UserID == "" || p.Content == "" {
		t.Error("NewTestPost returned empty required fields")
	}
}

func TestNewTestCommand(t *testing.T) {
	cmd := testutil.NewTestCommand()
	if cmd.ID == "" || cmd.Slug == "" || cmd.Description == "" {
		t.Error("NewTestCommand returned empty required fields")
	}
}

func TestNewTestThread(t *testing.T) {
	th := testutil.NewTestThread()
	if th.PostID == "" || th.ChannelID == "" || th.ReplyCount == 0 {
		t.Error("NewTestThread returned empty required fields")
	}
}

func TestMockServerMux(t *testing.T) {
	srv, mux := testutil.NewMockServerMux()
	defer srv.Close()

	mux.HandleFunc("/test", testutil.MockHandler(200, map[string]string{"ok": "true"}))

	resp, err := srv.Client().Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
