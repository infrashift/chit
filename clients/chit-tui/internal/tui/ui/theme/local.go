package theme

import (
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

// ErrLocalThemeNotFound reports that no user theme file exists under the themes
// directory. Callers distinguish it from a file that exists but is broken: the
// first means "try somewhere else", the second means "tell the user".
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var ErrLocalThemeNotFound = errors.New("local theme not found")

// namedTermColor maps the names a theme file may use to ANSI palette indices.
// A named color renders as whatever the user's terminal sets that slot to, so
// a theme can inherit an existing color scheme instead of overriding it.
//
// Two names are deliberately counter-intuitive, matching the ratatui palette
// these names come from: "gray" is ANSI 7 (the terminal's normal white) and
// "white" is ANSI 15 (bright white).
func namedTermColor(name string) (lipgloss.Color, bool) {
	switch name {
	case "black":
		return lipgloss.Color("0"), true
	case "red":
		return lipgloss.Color("1"), true
	case "green":
		return lipgloss.Color("2"), true
	case "yellow":
		return lipgloss.Color("3"), true
	case "blue":
		return lipgloss.Color("4"), true
	case "magenta":
		return lipgloss.Color("5"), true
	case "cyan":
		return lipgloss.Color("6"), true
	case "gray", "grey":
		return lipgloss.Color("7"), true
	case "darkgray", "dark_gray", "darkgrey", "dark_grey":
		return lipgloss.Color("8"), true
	case "lightred", "light_red":
		return lipgloss.Color("9"), true
	case "lightgreen", "light_green":
		return lipgloss.Color("10"), true
	case "lightyellow", "light_yellow":
		return lipgloss.Color("11"), true
	case "lightblue", "light_blue":
		return lipgloss.Color("12"), true
	case "lightmagenta", "light_magenta":
		return lipgloss.Color("13"), true
	case "lightcyan", "light_cyan":
		return lipgloss.Color("14"), true
	case "white":
		return lipgloss.Color("15"), true
	default:
		return "", false
	}
}

// parseThemeColor turns a validated theme-file value into a color. The schema
// has already rejected anything that is neither a hex literal nor a known
// name, so an unparseable value here yields nil and the slot falls back to the
// terminal default rather than failing the load.
func parseThemeColor(value string) color.Color {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "#") {
		return hexColor(value)
	}
	if c, ok := namedTermColor(strings.ToLower(value)); ok {
		assert.That(c != "", "parseThemeColor: known name mapped to an empty color")
		return c
	}
	return nil
}

// normalizeLocalThemeName lowercases a theme name and rejects anything that
// could escape the themes directory. The name reaches this function from a
// command line or a config file, so it is untrusted input used to build a path.
func normalizeLocalThemeName(name string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return "", errors.New("theme name is empty")
	}
	if strings.ContainsAny(trimmed, `/\`) || strings.Contains(trimmed, "..") ||
		trimmed == "." || filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("invalid theme name %q: names cannot contain path separators", name)
	}
	assert.That(filepath.Base(trimmed) == trimmed, "normalizeLocalThemeName: accepted name has a directory part")
	return trimmed, nil
}

// LoadLocal reads a user theme from themesDir/<name>.toml.
func LoadLocal(name, themesDir string) (Theme, []string, error) {
	normalized, err := normalizeLocalThemeName(name)
	if err != nil {
		return Theme{}, nil, err
	}

	path := filepath.Join(themesDir, normalized+".toml")
	assert.That(filepath.Dir(path) == filepath.Clean(themesDir), "LoadLocal: theme path escapes the themes directory")
	values, warnings, err := config.ValidateThemeTOML(path)
	if err != nil {
		if errors.Is(err, config.ErrThemeNotFound) {
			return Theme{}, warnings, ErrLocalThemeNotFound
		}
		return Theme{}, warnings, err
	}

	c := func(key string) color.Color { return parseThemeColor(values[key]) }

	t := Theme{
		Name:          values["name"],
		Author:        values["author"],
		Background:    c("background"),
		Foreground:    c("foreground"),
		Subtle:        c("subtle"),
		Accent:        c("accent"),
		Error:         c("error"),
		Success:       c("success"),
		Warning:       c("warning"),
		Border:        c("border"),
		ActiveBorder:  c("active_border"),
		Highlight:     c("highlight"),
		Muted:         c("muted"),
		Username:      c("username"),
		Timestamp:     c("timestamp"),
		UnreadBadge:   c("unread_badge"),
		PinBadge:      c("pin_badge"),
		ChannelActive: c("channel_active"),
		MentionBadge:  c("mention_badge"),
		MentionText:   c("mention_text"),
		MentionSelfBg: c("mention_self_bg"),
		TagBadge:      c("tag_badge"),

		// Absent unless declared, so derive() computes them from the palette
		// the file actually supplied.
		Selection:         parseThemeColor(values["selection"]),
		SearchMatch:       parseThemeColor(values["search_match"]),
		SearchMatchActive: parseThemeColor(values["search_match_active"]),
	}.derive()

	if t.Author == "" {
		t.Author = "local"
	}
	assert.That(t.Author != "", "LoadLocal: author left empty")
	return t, warnings, nil
}

// ResolveNamed resolves a theme name to a theme, preferring bundled themes over
// user files so a local file cannot shadow a name the documentation refers to.
// Legacy JSON skins are still honored, with a warning, so an existing
// ~/.config/chit/skins/<name>.json keeps working.
func ResolveNamed(name, themesDir string) (Theme, []string, error) {
	if t, ok := Lookup(name); ok {
		assert.That(t.Name != "", "ResolveNamed: bundled theme without a name")
		return t, nil, nil
	}

	t, warnings, err := LoadLocal(name, themesDir)
	if err == nil || !errors.Is(err, ErrLocalThemeNotFound) {
		return t, warnings, err
	}

	// Fall back to the pre-TOML skin format.
	if legacy, ok := loadLegacySkin(name); ok {
		assert.That(legacy.baseSet(), "ResolveNamed: legacy skin missing a defaulted slot")
		return legacy, append(warnings, fmt.Sprintf(
			"theme %q was loaded from the legacy skins directory; "+
				"move it to %s as a .toml file", name, themesDir)), nil
	}

	return Theme{}, warnings, ErrLocalThemeNotFound
}

// loadLegacySkin reads a theme from the JSON skins directory used before theme
// files moved to TOML.
func loadLegacySkin(name string) (Theme, bool) {
	normalized, err := normalizeLocalThemeName(name)
	if err != nil {
		return Theme{}, false
	}
	path := filepath.Join(skinsDir(), normalized+".json")
	assert.That(filepath.Dir(path) == skinsDir(), "loadLegacySkin: skin path escapes the skins directory")
	t, err := LoadFromFile(path)
	if err != nil {
		return Theme{}, false
	}
	assert.That(t.baseSet(), "loadLegacySkin: JSON theme missing a defaulted slot")
	return t, true
}
