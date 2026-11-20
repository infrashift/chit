//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
)

// These run the real TUI model against the real chitd: a Bubble Tea
// program with the real API and WebSocket clients, signed in as alice.

// tuiAsAlice starts the TUI for alice and opens town-square.
func tuiAsAlice(t *testing.T) *teatest.TestModel {
	t.Helper()
	return tuiAs(t, aliceKratosID)
}

// tuiAs starts the TUI for the user kratosID and opens town-square.
func tuiAs(t *testing.T, kratosID string) *teatest.TestModel {
	t.Helper()
	return tuiWith(t, kratosID, t.TempDir())
}

// tuiWith starts the TUI for kratosID, which saves files to downloadDir,
// and opens town-square.
func tuiWith(t *testing.T, kratosID, downloadDir string) *teatest.TestModel {
	t.Helper()
	client := newTestClient(kratosID)
	channelID := mustGetTownSquareID(t, client)
	ch, err := channelOf(client, channelID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ServerURL: serverURL(), DownloadDir: downloadDir}
	m := tui.NewModel(cfg, client, newTestWSClient(kratosID, 256), testutil.Styles(), nil, nil, nil)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 50))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Logf("quit: %v", err)
		}
	})
	tm.Send(palette.ChannelChosenMsg{Channel: ch})
	return tm
}

// channelOf finds channelID among alice's channels of uat-team.
func channelOf(c interface {
	GetMyTeams(context.Context) ([]*model.Team, error)
	GetMyChannels(context.Context, string) ([]*model.Channel, error)
}, channelID string) (*model.Channel, error) {
	ctx := context.Background()
	teams, err := c.GetMyTeams(ctx)
	if err != nil {
		return nil, err
	}
	for _, tm := range teams {
		chans, err := c.GetMyChannels(ctx, tm.ID)
		if err != nil {
			return nil, err
		}
		for _, ch := range chans {
			if ch.ID == channelID {
				return ch, nil
			}
		}
	}
	return nil, io.EOF
}

// waitScreen waits until the output since the last wait holds each of
// wants. A wait reads the output it checks, and the renderer draws only the
// lines that change, so one wait checks all that one screen must show.
func waitScreen(t *testing.T, tm *teatest.TestModel, wants ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		plain := testutil.StripANSI(string(out))
		for _, w := range wants {
			if !strings.Contains(plain, w) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(15*time.Second))
}

func TestTUI_ShowsTheHALOData(t *testing.T) {
	ctx := context.Background()
	agent := newTestClient(agentKratosID)
	alice := newTestClient(aliceKratosID)
	me := mustMe(t, alice)
	channelID := mustGetTownSquareID(t, alice)

	f, err := agent.UploadFile(ctx, channelID, "tui-e2e.png", "image/png", "", bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatalf("UploadFile (agent): %v", err)
	}
	if _, err := agent.CreatePost(ctx, &model.Post{ChannelID: channelID, Content: unique("agent chart"), FileIDs: []string{f.ID},
		Props: map[string]any{model.PropProvenance: map[string]any{"model": "claude-opus-5-5"}}}); err != nil {
		t.Fatalf("CreatePost (agent): %v", err)
	}
	status, body := rawDo(t, http.MethodPost, "/posts", map[string]string{"X-Client-Id": halodClientID, "X-On-Behalf-Of": me.ID},
		map[string]string{"channel_id": channelID, "content": unique("drafted for alice")})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("on-behalf-of post: %d %s", status, body)
	}

	tm := tuiAsAlice(t)
	waitScreen(t, tm, "[file] tui-e2e.png", "chit-agent [agent]", "[via @halod-tui-e2e]", "[provenance]")

	// A stream of the agent shows its indicator.
	time.Sleep(500 * time.Millisecond) // the WebSocket of the TUI connects
	status, body = rawDo(t, http.MethodPost, "/channels/"+channelID+"/stream_status",
		map[string]string{"X-User-Id": agentKratosID}, map[string]string{"status": model.StreamStarted, "stream_id": unique("s")})
	if status != http.StatusOK {
		t.Fatalf("stream_status: %d %s", status, body)
	}
	waitScreen(t, tm, "chit-agent is responding…")
}

// Bob's TUI sees alice's tag changes as they happen: a channel tag on the
// border and on a post, its rename (tag_updated), and its delete
// (tag_deleted, with no post_tags_updated after it).
func TestTUI_TagChangesArriveLive(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	name := strings.ReplaceAll(unique("live"), "-", "")
	tag, err := alice.CreateTag(ctx, name, model.TagScopeChannel, channelID)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	post, err := alice.CreatePost(ctx, &model.Post{ChannelID: channelID, Content: unique("tagged")})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if err := alice.AddTagToPost(ctx, post.ID, tag.ID); err != nil {
		t.Fatalf("AddTagToPost: %v", err)
	}

	tm := tuiAs(t, bobKratosID)
	waitScreen(t, tm, "#"+name)

	if err := alice.AddChannelTag(ctx, channelID, tag.ID); err != nil {
		t.Fatalf("AddChannelTag: %v", err)
	}
	waitScreen(t, tm, "── #"+name)

	renamed := name + "x"
	if _, err := alice.RenameTag(ctx, tag.ID, renamed); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	waitScreen(t, tm, "#"+renamed)

	if err := alice.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	marker := unique("after the delete")
	if _, err := alice.CreatePost(ctx, &model.Post{ChannelID: channelID, Content: marker}); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	waitScreen(t, tm, marker)
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	final := testutil.StripANSI(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View())
	if strings.Contains(final, "#"+renamed) {
		t.Errorf("a deleted tag still shows:\n%s", final)
	}
}

// Alice attaches an image in her TUI, sends it with no text, and saves it
// again from the post details. The server holds the post with the file,
// and the saved bytes match the sha256 of the server.
func TestTUI_AttachSendAndSave(t *testing.T) {
	ctx := context.Background()
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	dir, dl := t.TempDir(), t.TempDir()
	name := unique("tui-attach") + ".png"
	if err := os.WriteFile(filepath.Join(dir, name), pngBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}

	tm := tuiWith(t, aliceKratosID, dl)
	time.Sleep(time.Second) // the channel opens
	tm.Type("/attach " + filepath.Join(dir, name))
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "Alt text for "+name)
	tm.Type("a red line")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "backspace removes")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // the files alone
	waitScreen(t, tm, "[file] "+name+"  image/png")

	posts, err := alice.GetChannelPosts(ctx, channelID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	var fileID string
	for _, p := range posts.Order {
		if len(p.FileIDs) == 1 && p.Content == "" {
			if info, err := alice.GetFileInfo(ctx, p.FileIDs[0]); err == nil && info.Name == name {
				fileID = info.ID
				if info.AltText != "a red line" {
					t.Errorf("alt text = %q", info.AltText)
				}
			}
		}
	}
	if fileID == "" {
		t.Fatal("the server holds no post with the file")
	}

	// The cursor of the history stays where it was when a post arrives,
	// so it moves down to the newest post first.
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	for range 30 {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	waitScreen(t, tm, "save the files to")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	waitScreen(t, tm, "saved 1 file")
	data, err := os.ReadFile(filepath.Join(dl, name))
	if err != nil {
		t.Fatalf("the saved file: %v", err)
	}
	info, err := alice.GetFileInfo(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != info.SHA256 {
		t.Error("the saved bytes do not match the sha256 of the server")
	}
}
