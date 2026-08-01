package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
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
		_, _ = fmt.Fprintf(out, "Usage: chit-tui [flags]\n\nFlags:\n")
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(out, "\nConfiguration is read from %s.\n", config.FilePath())
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

	for _, name := range theme.ListAvailable() {
		if bundled[name] {
			_, _ = fmt.Fprintf(out, "%s\n", name)
			continue
		}
		_, _ = fmt.Fprintf(out, "%s (local)\n", name)
	}
}

// resolveTheme turns the command line and configuration into a theme.
func resolveTheme(opts options, cfg *config.Config) (theme.Theme, []string, error) {
	flagAppearance, err := theme.ParseAppearance(opts.appearance)
	if err != nil {
		return theme.Theme{}, nil, err
	}

	// A bad appearance in the config file is a warning, not a failure, so it
	// is parsed leniently here and simply left unset when invalid.
	cfgAppearance, _ := theme.ParseAppearance(cfg.Appearance)

	return theme.Resolve(theme.Request{
		FlagTheme:        opts.theme,
		ConfigTheme:      cfg.ThemeName,
		ConfigThemeDark:  cfg.ThemeDark,
		ConfigThemeLight: cfg.ThemeLight,
		FlagAppearance:   flagAppearance,
		ConfigAppearance: cfgAppearance,
		ThemesDir:        config.ThemesDir(),
		SystemIsDark:     detectSystemDark,
	})
}

// detectSystemDark asks the terminal whether it has a dark background.
//
// This must run before the Bubble Tea program starts: the query is an OSC
// escape sequence and its reply would otherwise arrive as input, which is the
// same class of leakage termResponseRe exists to filter. When stdin or stdout
// is not a terminal there is nobody to ask, and dark is the safer assumption.
func detectSystemDark() bool {
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}
	return lipgloss.HasDarkBackground()
}
