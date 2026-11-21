package tui_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/infobox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/prompt"
)

// Attachments (ISSUE-023): /attach, the tray, a post of files, and the
// save of the files of a post.

// attachModel opens c1 with the download directory in a temp directory.
func attachModel(t *testing.T) (tui.Model, *mockClient, string) {
	t.Helper()
	dl := t.TempDir()
	client := &mockClient{me: &model.User{ID: "u1", Username: "alice"}}
	cfg := &config.Config{ServerURL: "http://localhost:8065", DownloadDir: dl}
	m := tui.NewModel(cfg, client, nil, testutil.Styles(), nil, nil, nil)
	m.SetClipboardOutput(io.Discard)
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 120, Height: 40},
		tui.UserLoadedMsg{User: client.me},
		tui.TeamsLoadedMsg{Teams: []*model.Team{{ID: "t1"}}},
		tui.ChannelsLoadedMsg{TeamID: "t1", Channels: []*model.Channel{{ID: "c1", TeamID: "t1"}, {ID: "c2", TeamID: "t1"}}},
	} {
		m, _ = step(t, m, msg)
	}
	return m, client, dl
}

// imageFile writes an image file of size bytes and returns its path.
func imageFile(t *testing.T, name string, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// attach runs /attach path with the alt text, and feeds back the upload.
func attach(t *testing.T, m tui.Model, path, alt string) tui.Model {
	t.Helper()
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/attach " + path})
	if !strings.Contains(viewOf(m), "Alt text for "+filepath.Base(path)) {
		t.Fatalf("no alt text prompt:\n%s", viewOf(m))
	}
	m, cmd := step(t, m, prompt.SubmittedMsg{Key: "alt-text:" + path, Value: alt})
	return feed(t, m, cmd)
}

func TestAttach_UploadThenSendFilesAlone(t *testing.T) {
	m, client, _ := attachModel(t)
	path := imageFile(t, "chart.png", 100)
	m = attach(t, m, path, "a chart")
	if strings.Join(client.uploads, ",") != "c1/chart.png/image/png/a chart" {
		t.Errorf("uploads = %v", client.uploads)
	}
	if !strings.Contains(viewOf(m), "[file] chart.png") {
		t.Fatalf("no tray:\n%s", viewOf(m))
	}
	// Enter in an empty input sends the files alone.
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = step(t, m, wantMsg[input.SendMsg](t, cmd))
	created := wantMsg[tui.PostCreatedMsg](t, cmd)
	if p := client.lastCreatedPost; p == nil || !slices.Equal(p.FileIDs, []string{"f1"}) || p.Content != "" {
		t.Fatalf("post = %+v", client.lastCreatedPost)
	}
	m, _ = step(t, m, created)
	if strings.Contains(viewOf(m), "backspace removes") {
		t.Errorf("the tray outlived the post:\n%s", viewOf(m))
	}
	// The post shows the record of the upload, not a placeholder.
	if v := viewOf(m); !strings.Contains(v, "[file] chart.png  image/png") || strings.Contains(v, "[file f1]") {
		t.Errorf("the sent post does not show its file:\n%s", v)
	}
}

func TestAttach_RefusedPostKeepsTheTray(t *testing.T) {
	m, client, _ := attachModel(t)
	m = attach(t, m, imageFile(t, "a.png", 10), "")
	client.err = errors.New("server down")
	m, cmd := step(t, m, input.SendMsg{Content: "with a file"})
	m = feed(t, m, cmd)
	if !strings.Contains(viewOf(m), "[file] a.png") {
		t.Errorf("a refused post lost the tray:\n%s", viewOf(m))
	}
}

func TestAttach_Refusals(t *testing.T) {
	m, _, _ := attachModel(t)
	cases := map[string]string{
		imageFile(t, "notes.txt", 10):                  "attach a PNG, JPEG, WebP or GIF image",
		imageFile(t, "huge.png", model.MaxFileBytes+1): "too large",
		t.TempDir() + "/missing.png":                   "missing.png: no such file",
	}
	dir := filepath.Join(t.TempDir(), "dir.png")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cases[dir] = "is a directory"
	for path, want := range cases {
		got, _ := step(t, m, input.SlashTriggerMsg{Input: "/attach " + path})
		if !strings.Contains(viewOf(got), want) {
			t.Errorf("/attach %s: want %q:\n%s", filepath.Base(path), want, viewOf(got))
		}
	}
}

func TestAttach_TrayCapBackspaceDetachAndSwitch(t *testing.T) {
	m, _, _ := attachModel(t)
	for _, n := range []string{"1.png", "2.png", "3.png", "4.png"} {
		m = attach(t, m, imageFile(t, n, 10), "")
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/attach " + imageFile(t, "5.png", 10)})
	if !strings.Contains(viewOf(m), "at most 4 files") {
		t.Errorf("a fifth file was taken:\n%s", viewOf(m))
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if strings.Contains(viewOf(m), "[file] 4.png") || !strings.Contains(viewOf(m), "[file] 3.png") {
		t.Errorf("Backspace did not remove the last file:\n%s", viewOf(m))
	}
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/detach"})
	if strings.Contains(viewOf(m), "[file]") {
		t.Errorf("/detach left files:\n%s", viewOf(m))
	}
	m = attach(t, m, imageFile(t, "6.png", 10), "")
	m, _ = step(t, m, palette.ChannelChosenMsg{Channel: &model.Channel{ID: "c2", TeamID: "t1"}})
	if strings.Contains(viewOf(m), "[file]") {
		t.Errorf("the tray followed the user to another channel:\n%s", viewOf(m))
	}
}

func TestAttach_UploadErrorShows(t *testing.T) {
	m, client, _ := attachModel(t)
	client.err = &api.APIError{StatusCode: 413, AppError: &model.AppError{ID: "file.quota_exceeded"}}
	m = attach(t, m, imageFile(t, "q.png", 10), "")
	if !strings.Contains(viewOf(m), "q.png: your attachment quota is full") {
		t.Errorf("no reason for the refusal:\n%s", viewOf(m))
	}
}

func TestAttach_NoPathOpensThePicker(t *testing.T) {
	m, _, _ := attachModel(t)
	m, _ = step(t, m, input.SlashTriggerMsg{Input: "/attach"})
	if !strings.Contains(viewOf(m), "Attach an image") {
		t.Errorf("no file picker:\n%s", viewOf(m))
	}
}

func TestSave_TheFilesOfAPost(t *testing.T) {
	m, client, dl := attachModel(t)
	client.files = map[string]*model.File{"f1": {ID: "f1", Name: "../../evil.png"}}
	client.fileBytes = map[string][]byte{"f1": []byte("PNGDATA")}
	m, cmd := step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
		{ID: "p1", ChannelID: "c1", UserID: "u1", FileIDs: []string{"f1"}, CreateAt: 1700000000000},
	}}})
	m = feed(t, m, cmd)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	for range 2 {
		m, _ = step(t, m, key('i'))
		if !strings.Contains(viewOf(m), "s save the files to") {
			t.Fatalf("no save action:\n%s", viewOf(m))
		}
		m, cmd = step(t, m, key('s'))
		m, cmd = step(t, m, wantMsg[infobox.ActionMsg](t, cmd))
		m = feed(t, m, cmd)
	}
	for _, name := range []string{"evil.png", "evil (1).png"} {
		data, err := os.ReadFile(filepath.Join(dl, name))
		if err != nil || string(data) != "PNGDATA" {
			t.Errorf("%s = %q, %v", name, data, err)
		}
	}
	if !strings.Contains(viewOf(m), "saved 1 file to ") {
		t.Errorf("no notice:\n%s", viewOf(m))
	}
	if entries, err := os.ReadDir(filepath.Dir(dl)); err != nil || len(entries) > 1 {
		t.Errorf("a file escaped the download directory: %v, %v", entries, err)
	}
}

func TestSave_AFailedDownloadLeavesNoFile(t *testing.T) {
	m, client, dl := attachModel(t)
	client.files = map[string]*model.File{"f1": {ID: "f1", Name: "x.png"}}
	m, cmd := step(t, m, tui.PostsLoadedMsg{ChannelID: "c1", Posts: &model.PostList{Order: []*model.Post{
		{ID: "p1", ChannelID: "c1", UserID: "u1", FileIDs: []string{"f1"}, CreateAt: 1700000000000},
	}}})
	m = feed(t, m, cmd)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, key('i'))
	m, cmd = step(t, m, key('s'))
	m, cmd = step(t, m, wantMsg[infobox.ActionMsg](t, cmd))
	m = feed(t, m, cmd)
	if entries, err := os.ReadDir(dl); err != nil || len(entries) != 0 {
		t.Errorf("a failed download left %v, %v", entries, err)
	}
	if !strings.Contains(viewOf(m), "save:") {
		t.Errorf("no error:\n%s", viewOf(m))
	}
}
