package main

import (
	"context"
	"log"
	"os"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// termResponseRe matches terminal response sequences that leak into the input
// as KeyRunes messages: OSC replies (]11;rgb:...), cursor position reports
// (\d;\dR), and escape sequence fragments.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
var termResponseRe = regexp.MustCompile(
	`^\]\d|;rgb:|^\d+;\d+R$|^[0-9a-f]{4}\\\\$`,
)

func main() {
	// stderr reports problems found before the program starts. log.Logger
	// with no prefix and no flags writes the same text fmt.Fprintf did, and
	// it has no error to drop.
	stderr := log.New(os.Stderr, "", 0)

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
		stderr.Printf("warning: %s\n", w)
	}
	if err != nil {
		stderr.Printf("config error: %v\n", err)
		os.Exit(1)
	}
	assert.That(cfg != nil, "main: config.Load returned neither a config nor an error")

	// Resolved before the program starts: detecting terminal darkness queries
	// the terminal, and the reply would land in the input stream otherwise.
	// The one detector is shared so the terminal is asked at most once.
	background := &terminalBackground{}
	activeTheme, themeWarnings, err := resolveTheme(opts, cfg, background)
	for _, w := range themeWarnings {
		stderr.Printf("warning: %s\n", w)
	}
	if err != nil {
		stderr.Printf("theme error: %v\n", err)
		os.Exit(1)
	}

	m := newModel(cfg, styles.New(activeTheme))
	m.SetThemeSetting(themeSetting(opts, cfg, background))

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg { return filterInput(msg) }))
	if _, err := p.Run(); err != nil {
		stderr.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// newModel connects the clients and stores the configuration names and
// builds the root model on them.
func newModel(cfg *config.Config, s styles.Styles) tui.Model {
	assert.That(cfg != nil, "newModel: nil config")
	// config.Load validated these before main got here.
	assert.That(cfg.ServerURL != "", "newModel: no server URL")
	assert.That(cfg.AuthHeader != "", "newModel: no auth header")
	kratosClient := auth.NewKratosClient(cfg.KratosBaseURL())
	tokenStore := auth.NewTokenStore("")
	sessionStore := auth.NewSessionStore(cfg.SessionFile)

	// Pre-populate token: env var > stored session file > empty (triggers login).
	if cfg.HasToken() {
		// Env-sourced tokens skip validation.
		tokenStore.Set(cfg.SessionToken)
	} else {
		tokenStore.Set(storedToken(context.Background(), sessionStore, kratosClient))
	}
	assert.That(!cfg.HasToken() || tokenStore.Get() == cfg.SessionToken, "newModel: an environment token must take precedence")

	client := api.NewClientWithTokenSource(cfg.ServerURL, tokenStore, cfg.AuthHeader)
	wsClient := ws.NewWSClientWithHeader(cfg.WSURL(), tokenStore.Get(), 256, cfg.AuthHeader)

	return tui.NewModel(cfg, client, wsClient, s, tokenStore, kratosClient, sessionStore)
}

// sessionLoader reads the saved session; *auth.SessionStore is one.
type sessionLoader interface {
	Load() (*auth.StoredSession, error)
}

// sessionChecker asks Kratos whether a token is still a live session;
// *auth.KratosClient is one.
type sessionChecker interface {
	CheckSession(ctx context.Context, token string) (*auth.Identity, error)
}

// storedToken returns the saved session's token unless Kratos rejects it;
// "" sends the user to the sign-in screen. Only a rejection discards it: if
// Kratos cannot be reached, the session may well be fine, and the server
// will ask for sign-in itself if it is not.
func storedToken(ctx context.Context, store sessionLoader, checker sessionChecker) string {
	assert.That(ctx != nil, "storedToken: nil context")
	assert.That(store != nil, "storedToken: nil session loader")
	assert.That(checker != nil, "storedToken: nil session checker")
	stored, err := store.Load()
	if err != nil {
		return ""
	}
	assert.That(stored != nil, "storedToken: Load returned neither a session nor an error")
	if stored.Token == "" {
		return ""
	}
	if _, err := checker.CheckSession(ctx, stored.Token); err != nil && auth.IsSessionRejected(err) {
		return ""
	}
	return stored.Token
}
