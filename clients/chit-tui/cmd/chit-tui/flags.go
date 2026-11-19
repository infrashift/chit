package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// options holds everything the command line can set.
type options struct {
	theme      string
	appearance string
	listThemes bool
}

// parseFlags parses argv. It takes the output writer and arg slice so the
// parsing is testable without touching the real command line.
func parseFlags(out io.Writer, args []string) (options, error) {
	var opts options

	fs := flag.NewFlagSet("chit-tui", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.StringVar(&opts.theme, "theme", "",
		"theme name (bundled, or ~/.config/chit/themes/<name>.toml)")
	fs.StringVar(&opts.appearance, "appearance", "",
		"light, dark, or system (used when no explicit theme is set)")
	fs.BoolVar(&opts.listThemes, "list-themes", false,
		"list the available themes and exit")

	fs.Usage = func() {
		// The text is built in memory and written once, so one error check
		// covers it. PrintDefaults writes to the flag set's output, which is
		// pointed at the builder for the call.
		var b strings.Builder
		fmt.Fprintf(&b, "Usage: chit-tui [flags]\n\nFlags:\n")
		fs.SetOutput(&b)
		fs.PrintDefaults()
		fs.SetOutput(out)
		fmt.Fprintf(&b, "\nConfiguration is read from %s.\n", config.FilePath())
		_, err := io.WriteString(out, b.String())
		errlog.Debug(err, "write usage")
	}

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

// listThemes writes the available theme names, marking which are bundled.
func listThemes(out io.Writer) {
	bundled := make(map[string]bool)
	for _, n := range theme.BuiltinNames() {
		bundled[n] = true
	}

	var b strings.Builder
	for _, name := range theme.ListAvailable(config.ThemesDir()) {
		if bundled[name] {
			fmt.Fprintf(&b, "%s\n", name)
			continue
		}
		fmt.Fprintf(&b, "%s (local)\n", name)
	}
	_, err := io.WriteString(out, b.String())
	errlog.Debug(err, "write theme list")
}

// resolveTheme turns the command line and configuration into a theme.
func resolveTheme(opts options, cfg *config.Config) (theme.Theme, []string, error) {
	req, err := themeRequest(opts, cfg)
	if err != nil {
		return theme.Theme{}, nil, err
	}
	return theme.Resolve(req)
}

// themeSetting is the config key a theme picked in the client is saved under,
// which depends on the appearance in effect.
func themeSetting(opts options, cfg *config.Config) string {
	req, err := themeRequest(opts, cfg)
	if err != nil {
		return "theme"
	}
	return cfg.ThemeSettingKey(req.Dark())
}

func themeRequest(opts options, cfg *config.Config) (theme.Request, error) {
	flagAppearance, err := theme.ParseAppearance(opts.appearance)
	if err != nil {
		return theme.Request{}, err
	}

	// A bad appearance in the config file is a warning, not a failure, so it
	// is parsed leniently here and simply left unset when invalid.
	cfgAppearance, err := theme.ParseAppearance(cfg.Appearance)
	if err != nil {
		cfgAppearance = theme.AppearanceUnset
	}

	return theme.Request{
		FlagTheme:        opts.theme,
		ConfigTheme:      cfg.ThemeName,
		ConfigThemeDark:  cfg.ThemeDark,
		ConfigThemeLight: cfg.ThemeLight,
		FlagAppearance:   flagAppearance,
		ConfigAppearance: cfgAppearance,
		ThemesDir:        config.ThemesDir(),
		System:           terminalBackground{},
	}, nil
}

// terminalBackground is the theme.DarkDetector for the real terminal.
type terminalBackground struct{}

// IsDark reports whether the terminal has a dark background.
func (terminalBackground) IsDark() bool { return detectSystemDark() }

// detectSystemDark asks the terminal whether it has a dark background.
//
// This must run before the Bubble Tea program starts: the query is an OSC
// escape sequence and its reply would otherwise arrive as input, which is the
// same class of leakage termResponseRe exists to filter. When stdin or stdout
// is not a terminal there is nobody to ask, and dark is the safer assumption.
//
// It asks once: both the theme and where a picked theme is saved depend on
// the answer.
var detectSystemDark = sync.OnceValue(func() bool {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}
	return lipgloss.HasDarkBackground()
})
