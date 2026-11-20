//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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
	alice := newTestClient(aliceKratosID)
	channelID := mustGetTownSquareID(t, alice)
	ch, err := channelOf(alice, channelID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ServerURL: serverURL()}
	m := tui.NewModel(cfg, alice, newTestWSClient(aliceKratosID, 256), testutil.Styles(), nil, nil, nil)
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
