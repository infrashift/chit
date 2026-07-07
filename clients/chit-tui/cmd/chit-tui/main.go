package main

import (
	"context"
	"fmt"
	"os"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// termResponseRe matches terminal response sequences that leak into the input
// as KeyRunes messages: OSC replies (]11;rgb:...), cursor position reports
// (\d;\dR), and escape sequence fragments.
var termResponseRe = regexp.MustCompile(
	`^\]\d|;rgb:|^\d+;\d+R$|^[0-9a-f]{4}\\\\$`,
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	kratosClient := auth.NewKratosClient(cfg.KratosBaseURL())
	tokenStore := auth.NewTokenStore("")
	sessionStore := auth.NewSessionStore(cfg.SessionFile)

	// Pre-populate token: env var > stored session file > empty (triggers login).
	fromEnv := false
	if cfg.HasToken() {
		tokenStore.Set(cfg.SessionToken)
		fromEnv = true
	} else {
		if stored, err := sessionStore.Load(); err == nil && stored.Token != "" {
			// Validate stored token before using it.
			if _, err := kratosClient.CheckSession(context.Background(), stored.Token); err == nil {
				tokenStore.Set(stored.Token)
			}
			// If validation fails, token stays empty → login screen.
		}
	}
	_ = fromEnv // suppress unused warning; env-sourced tokens skip validation

	client := api.NewClientWithTokenFn(cfg.ServerURL, tokenStore.Get, cfg.AuthHeader)
	wsClient := ws.NewWSClientWithHeader(cfg.WSURL(), tokenStore.Get(), 256, cfg.AuthHeader)

	s := styles.New(theme.LoadNamed(cfg.ThemeName))
	m := tui.NewModel(cfg, client, wsClient, s, tokenStore, kratosClient, sessionStore)

	// Filter out terminal response sequences (OSC replies, cursor position
	// reports) that can leak into the textarea as garbage text.
	termFilter := func(_ tea.Model, msg tea.Msg) tea.Msg {
		if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyRunes {
			if termResponseRe.MatchString(string(k.Runes)) {
				return nil
			}
		}
		return msg
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithFilter(termFilter))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
