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

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	assert.That(out != nil, "parseFlags: nil output")
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
	assert.That(fs.Parsed(), "parseFlags: flags not parsed")
	return opts, nil
}

// listThemes writes the available theme names, marking which are bundled.
func listThemes(out io.Writer) {
	assert.That(out != nil, "listThemes: nil output")
	bundled := make(map[string]bool)
	for _, n := range theme.BuiltinNames() {
		bundled[n] = true
	}
	assert.That(len(bundled) > 0, "listThemes: no bundled themes")

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
// system answers whether the terminal is dark when the appearance is
// "system".
func resolveTheme(opts options, cfg *config.Config, system theme.DarkDetector) (theme.Theme, []string, error) {
	assert.That(cfg != nil, "resolveTheme: nil config")
	assert.That(system != nil, "resolveTheme: nil dark detector")
	req, err := themeRequest(opts, cfg, system)
	if err != nil {
		return theme.Theme{}, nil, err
	}
	return theme.Resolve(req)
}

// themeSetting is the config key a theme picked in the client is saved under,
// which depends on the appearance in effect.
func themeSetting(opts options, cfg *config.Config, system theme.DarkDetector) string {
	assert.That(cfg != nil, "themeSetting: nil config")
	assert.That(system != nil, "themeSetting: nil dark detector")
	req, err := themeRequest(opts, cfg, system)
	if err != nil {
		return "theme"
	}
	key := cfg.ThemeSettingKey(req.Dark())
	assert.That(key == "theme" || strings.HasPrefix(key, "theme_"), "themeSetting: not a theme setting key")
	return key
}

func themeRequest(opts options, cfg *config.Config, system theme.DarkDetector) (theme.Request, error) {
	assert.That(cfg != nil, "themeRequest: nil config")
	assert.That(system != nil, "themeRequest: nil dark detector")
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
		System:           system,
	}, nil
}

// terminalBackground is the theme.DarkDetector for the real terminal. main
// makes one and hands it to every theme lookup, so the terminal is asked at
// most once: both the theme and where a picked theme is saved depend on the
// answer, and the two must agree.
type terminalBackground struct {
	once sync.Once
	dark bool
}

// IsDark reports whether the terminal has a dark background. The first call
// asks the terminal; later calls return the same answer.
func (b *terminalBackground) IsDark() bool {
	assert.That(b != nil, "terminalBackground.IsDark: nil detector")
	b.once.Do(func() { b.dark = detectSystemDark() })
	return b.dark
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
