package theme

import (
	"image/color"
	"sort"
	"strings"
)

// flavor is a theme's upstream palette expressed in the vocabulary those
// palettes actually publish — a background, a few foreground weights, and the
// accent hues. themeFrom maps it onto chit's slots, so adding a variant of an
// existing family means supplying twelve colors rather than restating every
// slot. Ported from the flavor/generator split in mrman's theme package.
type flavor struct {
	name   string
	author string

	bg     color.Color // window background
	bgAlt  color.Color // raised surface: selected rows, status bar
	border color.Color // panel borders (often equal to bgAlt)

	fg      color.Color // primary text
	fgDim   color.Color // de-emphasized text: timestamps, hints
	fgMuted color.Color // furthest back: separators

	red     color.Color // errors, mentions of you
	green   color.Color // success, tags
	yellow  color.Color // warnings, pins
	blue    color.Color // primary accent, links, active channel
	magenta color.Color // usernames
	cyan    color.Color // unread badges
}

// themeFrom maps a flavor onto the full slot set. Slots that carry the same
// meaning share a hue deliberately: an unread badge and a pin badge are both
// "attention", so they track cyan and yellow rather than drifting apart per
// theme. derive() then fills the selection and search slots.
func themeFrom(f flavor) Theme {
	return Theme{
		Name:          f.name,
		Author:        f.author,
		Background:    f.bg,
		Foreground:    f.fg,
		Subtle:        f.fgDim,
		Accent:        f.blue,
		Error:         f.red,
		Success:       f.green,
		Warning:       f.yellow,
		Border:        f.border,
		ActiveBorder:  f.blue,
		Highlight:     f.bgAlt,
		Muted:         f.fgMuted,
		Username:      f.magenta,
		Timestamp:     f.fgDim,
		UnreadBadge:   f.cyan,
		PinBadge:      f.yellow,
		ChannelActive: f.blue,
		MentionBadge:  f.red,
		MentionText:   f.blue,
		MentionSelfBg: f.yellow,
		TagBadge:      f.green,
	}.derive()
}

// isDarkColor reports whether a color reads as dark, by channel average. A
// color with no RGB form (nil, ANSI index) counts as dark, matching the
// terminal default assumption used elsewhere.
func isDarkColor(c color.Color) bool {
	r, g, b, ok := rgbComponents(c)
	if !ok {
		return true
	}
	return (int(r)+int(g)+int(b))/3 < 128
}

// TokyoNight returns the default Tokyo Night theme.
func TokyoNight() Theme {
	return themeFrom(flavor{
		name: "Tokyo Night", author: "chit-tui",
		bg: hexColor("#1a1b26"), bgAlt: hexColor("#292e42"), border: hexColor("#3b4261"),
		fg: hexColor("#c0caf5"), fgDim: hexColor("#565f89"), fgMuted: hexColor("#545c7e"),
		red: hexColor("#f7768e"), green: hexColor("#9ece6a"), yellow: hexColor("#e0af68"),
		blue: hexColor("#7aa2f7"), magenta: hexColor("#bb9af7"), cyan: hexColor("#7aa2f7"),
	})
}

// TokyoNightDay returns the light Tokyo Night variant.
func TokyoNightDay() Theme {
	return themeFrom(flavor{
		name: "Tokyo Night Day", author: "folke",
		bg: hexColor("#e1e2e7"), bgAlt: hexColor("#c4c8da"), border: hexColor("#a8aecb"),
		fg: hexColor("#3760bf"), fgDim: hexColor("#6172b0"), fgMuted: hexColor("#848cb5"),
		red: hexColor("#c64343"), green: hexColor("#587539"), yellow: hexColor("#8c6c3e"),
		blue: hexColor("#2e7de9"), magenta: hexColor("#9854f1"), cyan: hexColor("#007197"),
	})
}

// Catppuccin returns the Catppuccin Mocha theme.
func Catppuccin() Theme {
	return themeFrom(flavor{
		name: "Catppuccin Mocha", author: "catppuccin",
		bg: hexColor("#1e1e2e"), bgAlt: hexColor("#313244"), border: hexColor("#313244"),
		fg: hexColor("#cdd6f4"), fgDim: hexColor("#6c7086"), fgMuted: hexColor("#585b70"),
		red: hexColor("#f38ba8"), green: hexColor("#a6e3a1"), yellow: hexColor("#f9e2af"),
		blue: hexColor("#89b4fa"), magenta: hexColor("#cba6f7"), cyan: hexColor("#89b4fa"),
	})
}

// CatppuccinLatte returns the light Catppuccin flavor.
func CatppuccinLatte() Theme {
	return themeFrom(flavor{
		name: "Catppuccin Latte", author: "catppuccin",
		bg: hexColor("#eff1f5"), bgAlt: hexColor("#ccd0da"), border: hexColor("#bcc0cc"),
		fg: hexColor("#4c4f69"), fgDim: hexColor("#6c6f85"), fgMuted: hexColor("#8c8fa1"),
		red: hexColor("#d20f39"), green: hexColor("#40a02b"), yellow: hexColor("#df8e1d"),
		blue: hexColor("#1e66f5"), magenta: hexColor("#8839ef"), cyan: hexColor("#179299"),
	})
}

// Kanagawa returns the Kanagawa Wave theme.
func Kanagawa() Theme {
	return themeFrom(flavor{
		name: "Kanagawa", author: "rebelot",
		bg: hexColor("#1f1f28"), bgAlt: hexColor("#2a2a37"), border: hexColor("#2a2a37"),
		fg: hexColor("#dcd7ba"), fgDim: hexColor("#727169"), fgMuted: hexColor("#54546d"),
		red: hexColor("#e82424"), green: hexColor("#98bb6c"), yellow: hexColor("#e6c384"),
		blue: hexColor("#7e9cd8"), magenta: hexColor("#957fb8"), cyan: hexColor("#7fb4ca"),
	})
}

// KanagawaLotus returns the light Kanagawa variant.
func KanagawaLotus() Theme {
	return themeFrom(flavor{
		name: "Kanagawa Lotus", author: "rebelot",
		bg: hexColor("#f2ecbc"), bgAlt: hexColor("#e5ddb0"), border: hexColor("#d5cea3"),
		fg: hexColor("#545464"), fgDim: hexColor("#8a8980"), fgMuted: hexColor("#9c9b90"),
		red: hexColor("#c84053"), green: hexColor("#6f894e"), yellow: hexColor("#77713f"),
		blue: hexColor("#4d699b"), magenta: hexColor("#624c83"), cyan: hexColor("#597b75"),
	})
}

// Nightfox returns the Nightfox theme.
func Nightfox() Theme {
	return themeFrom(flavor{
		name: "Nightfox", author: "EdenEast",
		bg: hexColor("#192330"), bgAlt: hexColor("#29394f"), border: hexColor("#29394f"),
		fg: hexColor("#cdcecf"), fgDim: hexColor("#71839b"), fgMuted: hexColor("#575860"),
		red: hexColor("#c94f6d"), green: hexColor("#81b29a"), yellow: hexColor("#dbc074"),
		blue: hexColor("#719cd6"), magenta: hexColor("#9d79d6"), cyan: hexColor("#63cdcf"),
	})
}

// Dayfox returns the light Nightfox variant.
func Dayfox() Theme {
	return themeFrom(flavor{
		name: "Dayfox", author: "EdenEast",
		bg: hexColor("#f6f2ee"), bgAlt: hexColor("#e7d2be"), border: hexColor("#d3c1ae"),
		fg: hexColor("#3d2b5a"), fgDim: hexColor("#6e6a86"), fgMuted: hexColor("#8b8a95"),
		red: hexColor("#a5222f"), green: hexColor("#396847"), yellow: hexColor("#ac5402"),
		blue: hexColor("#2848a9"), magenta: hexColor("#6e33ce"), cyan: hexColor("#287980"),
	})
}

// builtinThemes maps names to built-in theme constructors. Each call builds a
// fresh Theme so callers can mutate what they get back.
var builtinThemes = map[string]func() Theme{
	"tokyo-night":      TokyoNight,
	"tokyo-night-day":  TokyoNightDay,
	"catppuccin":       Catppuccin,
	"catppuccin-latte": CatppuccinLatte,
	"kanagawa":         Kanagawa,
	"kanagawa-lotus":   KanagawaLotus,
	"nightfox":         Nightfox,
	"dayfox":           Dayfox,
}

// Lookup resolves a bundled theme by name, ignoring case and surrounding
// whitespace so a value typed on the command line or pasted into a config file
// behaves the same way.
func Lookup(name string) (Theme, bool) {
	ctor, ok := builtinThemes[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return Theme{}, false
	}
	return ctor(), true
}

// BuiltinNames returns the bundled theme names in sorted order.
func BuiltinNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
