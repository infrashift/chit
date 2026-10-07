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
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// termResponseRe matches terminal response sequences that leak into the input
// as KeyRunes messages: OSC replies (]11;rgb:...), cursor position reports
// (\d;\dR), and escape sequence fragments.
var termResponseRe = regexp.MustCompile(
	`^\]\d|;rgb:|^\d+;\d+R$|^[0-9a-f]{4}\\\\$`,
)

func main() {
	opts, err := parseFlags(os.Stderr, os.Args[1:])
	if err != nil {
		// flag already reported the problem and printed usage.
		os.Exit(2)
	}

	if opts.listThemes {
		listThemes(os.Stdout)
		return
	}

	cfg, warnings, err := config.Load()
	// Warnings describe settings that were ignored. They are printed even when
	// the load then fails, since an ignored setting is often the reason.
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Resolved before the program starts: detecting terminal darkness queries
	// the terminal, and the reply would land in the input stream otherwise.
	activeTheme, themeWarnings, err := resolveTheme(opts, cfg)
	for _, w := range themeWarnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "theme error: %v\n", err)
		os.Exit(1)
	}

	kratosClient := auth.NewKratosClient(cfg.KratosBaseURL())
	tokenStore := auth.NewTokenStore("")
	sessionStore := auth.NewSessionStore(cfg.SessionFile)

	// Pre-populate token: env var > stored session file > empty (triggers login).
	if cfg.HasToken() {
		// Env-sourced tokens skip validation.
		tokenStore.Set(cfg.SessionToken)
	} else {
		if stored, err := sessionStore.Load(); err == nil && stored.Token != "" {
			// Validate stored token before using it.
			if _, err := kratosClient.CheckSession(context.Background(), stored.Token); err == nil {
				tokenStore.Set(stored.Token)
			}
			// If validation fails, token stays empty → login screen.
		}
	}

	client := api.NewClientWithTokenFn(cfg.ServerURL, tokenStore.Get, cfg.AuthHeader)
	wsClient := ws.NewWSClientWithHeader(cfg.WSURL(), tokenStore.Get(), 256, cfg.AuthHeader)

	s := styles.New(activeTheme)
	m := tui.NewModel(cfg, client, wsClient, s, tokenStore, kratosClient, sessionStore)
	m.SetThemeSetting(themeSetting(opts, cfg))

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
