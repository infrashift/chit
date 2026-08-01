package theme

import (
	"image/color"
	"sort"
	"testing"
)

// slots renders every palette slot as hex so themes can be compared as data.
func slots(t Theme) map[string]string {
	return map[string]string{
		"Background":    colorToHex(t.Background),
		"Foreground":    colorToHex(t.Foreground),
		"Subtle":        colorToHex(t.Subtle),
		"Accent":        colorToHex(t.Accent),
		"Error":         colorToHex(t.Error),
		"Success":       colorToHex(t.Success),
		"Warning":       colorToHex(t.Warning),
		"Border":        colorToHex(t.Border),
		"ActiveBorder":  colorToHex(t.ActiveBorder),
		"Highlight":     colorToHex(t.Highlight),
		"Muted":         colorToHex(t.Muted),
		"Username":      colorToHex(t.Username),
		"Timestamp":     colorToHex(t.Timestamp),
		"UnreadBadge":   colorToHex(t.UnreadBadge),
		"PinBadge":      colorToHex(t.PinBadge),
		"ChannelActive": colorToHex(t.ChannelActive),
		"MentionBadge":  colorToHex(t.MentionBadge),
		"MentionText":   colorToHex(t.MentionText),
		"MentionSelfBg": colorToHex(t.MentionSelfBg),
		"TagBadge":      colorToHex(t.TagBadge),
	}
}

// Routing the bundled themes through a flavor generator must not change what
// users already see. These are the exact values the hand-written constructors
// produced before the generator existed.
func TestLegacyThemesAreUnchanged(t *testing.T) {
	tests := []struct {
		name string
		got  Theme
		want map[string]string
	}{
		{
			name: "tokyo-night",
			got:  TokyoNight(),
			want: map[string]string{
				"Background": "#1a1b26", "Foreground": "#c0caf5", "Subtle": "#565f89",
				"Accent": "#7aa2f7", "Error": "#f7768e", "Success": "#9ece6a",
				"Warning": "#e0af68", "Border": "#3b4261", "ActiveBorder": "#7aa2f7",
				"Highlight": "#292e42", "Muted": "#545c7e", "Username": "#bb9af7",
				"Timestamp": "#565f89", "UnreadBadge": "#7aa2f7", "PinBadge": "#e0af68",
				"ChannelActive": "#7aa2f7", "MentionBadge": "#f7768e",
				"MentionText": "#7aa2f7", "MentionSelfBg": "#e0af68", "TagBadge": "#9ece6a",
			},
		},
		{
			name: "catppuccin",
			got:  Catppuccin(),
			want: map[string]string{
				"Background": "#1e1e2e", "Foreground": "#cdd6f4", "Subtle": "#6c7086",
				"Accent": "#89b4fa", "Error": "#f38ba8", "Success": "#a6e3a1",
				"Warning": "#f9e2af", "Border": "#313244", "ActiveBorder": "#89b4fa",
				"Highlight": "#313244", "Muted": "#585b70", "Username": "#cba6f7",
				"Timestamp": "#6c7086", "UnreadBadge": "#89b4fa", "PinBadge": "#f9e2af",
				"ChannelActive": "#89b4fa", "MentionBadge": "#f38ba8",
				"MentionText": "#89b4fa", "MentionSelfBg": "#f9e2af", "TagBadge": "#a6e3a1",
			},
		},
		{
			name: "kanagawa",
			got:  Kanagawa(),
			want: map[string]string{
				"Background": "#1f1f28", "Foreground": "#dcd7ba", "Subtle": "#727169",
				"Accent": "#7e9cd8", "Error": "#e82424", "Success": "#98bb6c",
				"Warning": "#e6c384", "Border": "#2a2a37", "ActiveBorder": "#7e9cd8",
				"Highlight": "#2a2a37", "Muted": "#54546d", "Username": "#957fb8",
				"Timestamp": "#727169", "UnreadBadge": "#7fb4ca", "PinBadge": "#e6c384",
				"ChannelActive": "#7e9cd8", "MentionBadge": "#e82424",
				"MentionText": "#7e9cd8", "MentionSelfBg": "#e6c384", "TagBadge": "#98bb6c",
			},
		},
		{
			name: "nightfox",
			got:  Nightfox(),
			want: map[string]string{
				"Background": "#192330", "Foreground": "#cdcecf", "Subtle": "#71839b",
				"Accent": "#719cd6", "Error": "#c94f6d", "Success": "#81b29a",
				"Warning": "#dbc074", "Border": "#29394f", "ActiveBorder": "#719cd6",
				"Highlight": "#29394f", "Muted": "#575860", "Username": "#9d79d6",
				"Timestamp": "#71839b", "UnreadBadge": "#63cdcf", "PinBadge": "#dbc074",
				"ChannelActive": "#719cd6", "MentionBadge": "#c94f6d",
				"MentionText": "#719cd6", "MentionSelfBg": "#dbc074", "TagBadge": "#81b29a",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slots(tc.got)
			for slot, want := range tc.want {
				if got[slot] != want {
					t.Errorf("%s = %s, want %s", slot, got[slot], want)
				}
			}
		})
	}
}

func TestEveryBuiltinPopulatesEverySlot(t *testing.T) {
	for name, ctor := range builtinThemes {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			for slot, hex := range slots(th) {
				if hex == "" {
					t.Errorf("%s is unset", slot)
				}
			}
			if th.Name == "" {
				t.Error("Name is empty")
			}
			if th.Author == "" {
				t.Error("Author is empty")
			}
		})
	}
}

func TestLookupIsCaseInsensitiveAndTrims(t *testing.T) {
	for _, name := range []string{"tokyo-night", "TOKYO-NIGHT", "  Tokyo-Night  "} {
		th, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) not found", name)
		}
		if colorToHex(th.Background) != "#1a1b26" {
			t.Errorf("Lookup(%q) returned the wrong theme", name)
		}
	}
}

func TestLookupRejectsUnknown(t *testing.T) {
	if _, ok := Lookup("no-such-theme"); ok {
		t.Error("Lookup reported an unknown theme as found")
	}
}

// Lookup must hand back an independent value each call; callers mutate themes
// (a transparent background, say) and must not corrupt the bundled palette.
func TestLookupReturnsFreshInstance(t *testing.T) {
	first, _ := Lookup("tokyo-night")
	first.Background = hexColor("#ff0000")

	second, _ := Lookup("tokyo-night")
	if colorToHex(second.Background) != "#1a1b26" {
		t.Errorf("second Lookup saw a mutated palette: %s", colorToHex(second.Background))
	}
}

func TestBuiltinNamesIsSortedAndComplete(t *testing.T) {
	names := BuiltinNames()
	if len(names) != len(builtinThemes) {
		t.Fatalf("BuiltinNames returned %d names, want %d", len(names), len(builtinThemes))
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("BuiltinNames is not sorted: %v", names)
	}
	for _, n := range names {
		if _, ok := builtinThemes[n]; !ok {
			t.Errorf("BuiltinNames returned %q, which is not a builtin", n)
		}
	}
}

// Light variants exist so the picker is useful on a light terminal; a "light"
// theme whose background is dark would defeat the point.
func TestLightVariantsHaveLightBackgrounds(t *testing.T) {
	light := []string{"tokyo-night-day", "catppuccin-latte", "kanagawa-lotus", "dayfox"}
	for _, name := range light {
		t.Run(name, func(t *testing.T) {
			th, ok := Lookup(name)
			if !ok {
				t.Fatalf("%s is not registered", name)
			}
			if isDarkColor(th.Background) {
				t.Errorf("background %s is dark", colorToHex(th.Background))
			}
			if !isDarkColor(th.Foreground) {
				t.Errorf("foreground %s is light; text would be unreadable",
					colorToHex(th.Foreground))
			}
		})
	}
}

func TestDarkVariantsHaveDarkBackgrounds(t *testing.T) {
	dark := []string{"tokyo-night", "catppuccin", "kanagawa", "nightfox"}
	for _, name := range dark {
		t.Run(name, func(t *testing.T) {
			th, _ := Lookup(name)
			if !isDarkColor(th.Background) {
				t.Errorf("background %s is light", colorToHex(th.Background))
			}
		})
	}
}

func TestIsDarkColor(t *testing.T) {
	tests := []struct {
		name string
		in   color.Color
		want bool
	}{
		{name: "black", in: hexColor("#000000"), want: true},
		{name: "white", in: hexColor("#ffffff"), want: false},
		{name: "tokyo night bg", in: hexColor("#1a1b26"), want: true},
		{name: "latte bg", in: hexColor("#eff1f5"), want: false},
		{name: "nil counts as dark", in: nil, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDarkColor(tc.in); got != tc.want {
				t.Errorf("isDarkColor(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
