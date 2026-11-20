//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// The live check against the full stack (make kube-up, then
// tests/e2e/test_e2e.sh for the Kratos users): Kratos, Oathkeeper, chitd,
// Keto and SeaweedFS. The TUI signs in on its own sign-in screen, as a
// person does, and every request goes through Oathkeeper.
//
// It runs when CHIT_LIVE_PROXY_URL names the Oathkeeper proxy, for example
// http://localhost:4455. The suite in the e2e pod does not set it.

const (
	liveUser     = "alice@test.local"
	livePassword = "TestPass1234"
)

func liveProxy(t *testing.T) string {
	t.Helper()
	proxy := os.Getenv("CHIT_LIVE_PROXY_URL")
	if proxy == "" {
		t.Skip("CHIT_LIVE_PROXY_URL is not set: no full stack to check")
	}
	return strings.TrimRight(proxy, "/")
}

// liveTUI builds the TUI as main does, on the sign-in screen, with the
// config file and the downloads in temp directories.
func liveTUI(t *testing.T, proxy, downloadDir string) *teatest.TestModel {
	t.Helper()
	t.Setenv("CHIT_CONFIG_FILE", filepath.Join(t.TempDir(), "config.toml"))
	cfg := &config.Config{
		ServerURL: proxy, WSScheme: "ws", AuthHeader: config.DefaultAuthHeader,
		SessionFile: filepath.Join(t.TempDir(), "session.json"), DownloadDir: downloadDir,
	}
	tokens := auth.NewTokenStore("")
	client := api.NewClientWithTokenSource(cfg.ServerURL, tokens, cfg.AuthHeader)
	wsClient := ws.NewWSClientWithHeader(cfg.WSURL(), tokens.Get(), 256, cfg.AuthHeader)
	m := tui.NewModel(cfg, client, wsClient, testutil.Styles(), tokens,
		auth.NewKratosClient(cfg.KratosBaseURL()), auth.NewSessionStore(cfg.SessionFile))
	m.SetActiveTheme(mustTheme(t))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 50))
	t.Cleanup(func() {
		if err := tm.Quit(); err != nil {
			t.Logf("quit: %v", err)
		}
	})
	return tm
}

// liveToken signs in through Oathkeeper with the Kratos API, for the
// requests that the test makes beside the TUI.
func liveToken(t *testing.T, proxy string) string {
	t.Helper()
	kc := auth.NewKratosClient(proxy + "/kratos")
	ctx := context.Background()
	flow, err := kc.InitLoginFlow(ctx)
	if err != nil {
		t.Fatalf("Kratos login flow: %v", err)
	}
	s, err := kc.SubmitLogin(ctx, flow.ID, liveUser, livePassword)
	if err != nil || s.Token == "" {
		t.Fatalf("Kratos login: %v", err)
	}
	return s.Token
}

func TestLive_SignInAttachTagAndTheme(t *testing.T) {
	proxy := liveProxy(t)
	ctx := context.Background()
	alice := api.NewClient(proxy, liveToken(t, proxy))
	me, err := alice.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe through Oathkeeper: %v", err)
	}

	dl := t.TempDir()
	tm := liveTUI(t, proxy, dl)
	waitScreen(t, tm, "Chit Login")
	tm.Type(liveUser)
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	tm.Type(livePassword)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "○ "+me.Username)

	// Open alice's first channel.
	channelID := firstChannelOf(t, alice)
	ch, err := channelOf(alice, channelID)
	if err != nil {
		t.Fatal(err)
	}
	tm.Send(palette.ChannelChosenMsg{Channel: ch})

	// Attach and send through Oathkeeper to SeaweedFS.
	img := filepath.Join(t.TempDir(), unique("live")+".png")
	if err := os.WriteFile(img, pngBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	tm.Type("/attach " + img)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "Alt text for")
	tm.Type("live alt")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "backspace removes")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitScreen(t, tm, "[file] "+filepath.Base(img)+"  image/png")

	// A channel tag of alice shows on the border, and its rename arrives.
	name := strings.ReplaceAll(unique("live"), "-", "")
	tag, err := alice.CreateTag(ctx, name, model.TagScopeChannel, channelID)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if err := alice.AddChannelTag(ctx, channelID, tag.ID); err != nil {
		t.Fatalf("AddChannelTag: %v", err)
	}
	waitScreen(t, tm, "── #"+name)
	if _, err := alice.RenameTag(ctx, tag.ID, name+"x"); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	waitScreen(t, tm, "#"+name+"x")

	// A theme change at run time, by name.
	tm.Type("/theme kanagawa")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	final := testutil.StripANSI(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View())
	if strings.Contains(final, "no theme") || strings.Contains(final, "Chit Login") {
		t.Errorf("the final screen:\n%s", final)
	}
	if err := alice.DeleteTag(ctx, tag.ID); err != nil {
		t.Logf("clean up the tag: %v", err)
	}
}

// mustTheme returns the default theme, which main would resolve.
func mustTheme(t *testing.T) theme.Theme {
	t.Helper()
	th, ok := theme.Lookup("tokyo-night")
	if !ok {
		t.Fatal("no bundled theme tokyo-night")
	}
	return th
}

// firstChannelOf returns the first channel of the first team of c, which
// is the channel that the TUI opens.
func firstChannelOf(t *testing.T, c api.ChitClient) string {
	t.Helper()
	ctx := context.Background()
	teams, err := c.GetMyTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatalf("GetMyTeams = %v, %v", teams, err)
	}
	chans, err := c.GetMyChannels(ctx, teams[0].ID)
	if err != nil || len(chans) == 0 {
		t.Fatalf("GetMyChannels = %v, %v", chans, err)
	}
	return chans[0].ID
}
