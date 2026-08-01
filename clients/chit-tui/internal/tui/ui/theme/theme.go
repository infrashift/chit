package theme

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Theme defines all colors used throughout the TUI. Colors are
// image/color.Color so palettes can be derived arithmetically; a nil slot means
// "terminal default" and the style helpers skip the attribute entirely.
type Theme struct {
	Name          string
	Author        string
	Background    color.Color
	Foreground    color.Color
	Subtle        color.Color
	Accent        color.Color
	Error         color.Color
	Success       color.Color
	Warning       color.Color
	Border        color.Color
	ActiveBorder  color.Color
	Highlight     color.Color
	Muted         color.Color
	Username      color.Color
	Timestamp     color.Color
	UnreadBadge   color.Color
	PinBadge      color.Color
	ChannelActive color.Color
	MentionBadge  color.Color
	MentionText   color.Color
	MentionSelfBg color.Color
	TagBadge      color.Color

	// Derived slots. Left unset by the built-in constructors and computed by
	// derive() from the base palette, so a theme only has to declare the
	// colors it actually cares about. A theme file may still set them.
	Selection         color.Color
	SearchMatch       color.Color
	SearchMatchActive color.Color
}

// derive fills any unset derived slot from the base palette. It is idempotent,
// so a theme file that sets a slot explicitly keeps its value.
func (t Theme) derive() Theme {
	if t.Selection == nil {
		t.Selection = Blend(t.Background, t.Accent, 25)
	}
	if t.SearchMatch == nil {
		t.SearchMatch = Blend(t.Background, t.Warning, 30)
	}
	if t.SearchMatchActive == nil {
		t.SearchMatchActive = Blend(t.Background, t.Warning, 60)
	}
	return t
}

// themeFile is the JSON representation of a theme file.
type themeFile struct {
	Name              string `json:"name"`
	Author            string `json:"author"`
	Background        string `json:"background"`
	Foreground        string `json:"foreground"`
	Subtle            string `json:"subtle"`
	Accent            string `json:"accent"`
	Error             string `json:"error"`
	Success           string `json:"success"`
	Warning           string `json:"warning"`
	Border            string `json:"border"`
	ActiveBorder      string `json:"active_border"`
	Highlight         string `json:"highlight"`
	Muted             string `json:"muted"`
	Username          string `json:"username"`
	Timestamp         string `json:"timestamp"`
	UnreadBadge       string `json:"unread_badge"`
	PinBadge          string `json:"pin_badge"`
	ChannelActive     string `json:"channel_active"`
	MentionBadge      string `json:"mention_badge"`
	MentionText       string `json:"mention_text"`
	MentionSelfBg     string `json:"mention_self_bg"`
	TagBadge          string `json:"tag_badge"`
	Selection         string `json:"selection"`
	SearchMatch       string `json:"search_match"`
	SearchMatchActive string `json:"search_match_active"`
}

// TokyoNight returns the default Tokyo Night theme.
func TokyoNight() Theme {
	return Theme{
		Name:          "Tokyo Night",
		Author:        "chit-tui",
		Background:    hexColor("#1a1b26"),
		Foreground:    hexColor("#c0caf5"),
		Subtle:        hexColor("#565f89"),
		Accent:        hexColor("#7aa2f7"),
		Error:         hexColor("#f7768e"),
		Success:       hexColor("#9ece6a"),
		Warning:       hexColor("#e0af68"),
		Border:        hexColor("#3b4261"),
		ActiveBorder:  hexColor("#7aa2f7"),
		Highlight:     hexColor("#292e42"),
		Muted:         hexColor("#545c7e"),
		Username:      hexColor("#bb9af7"),
		Timestamp:     hexColor("#565f89"),
		UnreadBadge:   hexColor("#7aa2f7"),
		PinBadge:      hexColor("#e0af68"),
		ChannelActive: hexColor("#7aa2f7"),
		MentionBadge:  hexColor("#f7768e"),
		MentionText:   hexColor("#7aa2f7"),
		MentionSelfBg: hexColor("#e0af68"),
		TagBadge:      hexColor("#9ece6a"),
	}.derive()
}

// Catppuccin returns the Catppuccin Mocha theme.
func Catppuccin() Theme {
	return Theme{
		Name:          "Catppuccin Mocha",
		Author:        "catppuccin",
		Background:    hexColor("#1e1e2e"),
		Foreground:    hexColor("#cdd6f4"),
		Subtle:        hexColor("#6c7086"),
		Accent:        hexColor("#89b4fa"),
		Error:         hexColor("#f38ba8"),
		Success:       hexColor("#a6e3a1"),
		Warning:       hexColor("#f9e2af"),
		Border:        hexColor("#313244"),
		ActiveBorder:  hexColor("#89b4fa"),
		Highlight:     hexColor("#313244"),
		Muted:         hexColor("#585b70"),
		Username:      hexColor("#cba6f7"),
		Timestamp:     hexColor("#6c7086"),
		UnreadBadge:   hexColor("#89b4fa"),
		PinBadge:      hexColor("#f9e2af"),
		ChannelActive: hexColor("#89b4fa"),
		MentionBadge:  hexColor("#f38ba8"),
		MentionText:   hexColor("#89b4fa"),
		MentionSelfBg: hexColor("#f9e2af"),
		TagBadge:      hexColor("#a6e3a1"),
	}.derive()
}

// Kanagawa returns the Kanagawa theme.
func Kanagawa() Theme {
	return Theme{
		Name:          "Kanagawa",
		Author:        "rebelot",
		Background:    hexColor("#1f1f28"),
		Foreground:    hexColor("#dcd7ba"),
		Subtle:        hexColor("#727169"),
		Accent:        hexColor("#7e9cd8"),
		Error:         hexColor("#e82424"),
		Success:       hexColor("#98bb6c"),
		Warning:       hexColor("#e6c384"),
		Border:        hexColor("#2a2a37"),
		ActiveBorder:  hexColor("#7e9cd8"),
		Highlight:     hexColor("#2a2a37"),
		Muted:         hexColor("#54546d"),
		Username:      hexColor("#957fb8"),
		Timestamp:     hexColor("#727169"),
		UnreadBadge:   hexColor("#7fb4ca"),
		PinBadge:      hexColor("#e6c384"),
		ChannelActive: hexColor("#7e9cd8"),
		MentionBadge:  hexColor("#e82424"),
		MentionText:   hexColor("#7e9cd8"),
		MentionSelfBg: hexColor("#e6c384"),
		TagBadge:      hexColor("#98bb6c"),
	}.derive()
}

// Nightfox returns the Nightfox theme.
func Nightfox() Theme {
	return Theme{
		Name:          "Nightfox",
		Author:        "EdenEast",
		Background:    hexColor("#192330"),
		Foreground:    hexColor("#cdcecf"),
		Subtle:        hexColor("#71839b"),
		Accent:        hexColor("#719cd6"),
		Error:         hexColor("#c94f6d"),
		Success:       hexColor("#81b29a"),
		Warning:       hexColor("#dbc074"),
		Border:        hexColor("#29394f"),
		ActiveBorder:  hexColor("#719cd6"),
		Highlight:     hexColor("#29394f"),
		Muted:         hexColor("#575860"),
		Username:      hexColor("#9d79d6"),
		Timestamp:     hexColor("#71839b"),
		UnreadBadge:   hexColor("#63cdcf"),
		PinBadge:      hexColor("#dbc074"),
		ChannelActive: hexColor("#719cd6"),
		MentionBadge:  hexColor("#c94f6d"),
		MentionText:   hexColor("#719cd6"),
		MentionSelfBg: hexColor("#dbc074"),
		TagBadge:      hexColor("#81b29a"),
	}.derive()
}

// LoadFromFile loads a theme from a JSON file.
func LoadFromFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	return ParseJSON(data)
}

// ParseJSON parses a theme from JSON bytes.
func ParseJSON(data []byte) (Theme, error) {
	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return Theme{}, err
	}
	return tf.toTheme(), nil
}

// builtinThemes maps names to built-in theme constructors.
var builtinThemes = map[string]func() Theme{
	"tokyo-night": TokyoNight,
	"catppuccin":  Catppuccin,
	"kanagawa":    Kanagawa,
	"nightfox":    Nightfox,
}

// ListAvailable returns sorted names of all available themes.
// It includes built-in theme keys plus basenames (without .json) of any
// files found in ~/.config/chit/skins/.
func ListAvailable() []string {
	names := make([]string, 0, len(builtinThemes))
	for k := range builtinThemes {
		names = append(names, k)
	}
	sort.Strings(names)

	dir := skinsDir()
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			names = append(names, name)
		}
	}
	return names
}

// LoadNamed resolves a theme by name. It checks built-in themes first,
// then looks for ~/.config/chit/skins/<name>.json.
// Returns the default TokyoNight theme if nothing matches.
func LoadNamed(name string) Theme {
	if name == "" {
		return TokyoNight()
	}
	if fn, ok := builtinThemes[name]; ok {
		return fn()
	}
	dir := skinsDir()
	path := filepath.Join(dir, name+".json")
	t, err := LoadFromFile(path)
	if err != nil {
		return TokyoNight()
	}
	return t
}

func skinsDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "chit", "skins")
	}
	return filepath.Join(configDir, "chit", "skins")
}

func colorOrDefault(val string, def color.Color) color.Color {
	if c := hexColor(val); c != nil {
		return c
	}
	return def
}

func (tf themeFile) toTheme() Theme {
	d := TokyoNight()
	return Theme{
		Name:          tf.Name,
		Author:        tf.Author,
		Background:    colorOrDefault(tf.Background, d.Background),
		Foreground:    colorOrDefault(tf.Foreground, d.Foreground),
		Subtle:        colorOrDefault(tf.Subtle, d.Subtle),
		Accent:        colorOrDefault(tf.Accent, d.Accent),
		Error:         colorOrDefault(tf.Error, d.Error),
		Success:       colorOrDefault(tf.Success, d.Success),
		Warning:       colorOrDefault(tf.Warning, d.Warning),
		Border:        colorOrDefault(tf.Border, d.Border),
		ActiveBorder:  colorOrDefault(tf.ActiveBorder, d.ActiveBorder),
		Highlight:     colorOrDefault(tf.Highlight, d.Highlight),
		Muted:         colorOrDefault(tf.Muted, d.Muted),
		Username:      colorOrDefault(tf.Username, d.Username),
		Timestamp:     colorOrDefault(tf.Timestamp, d.Timestamp),
		UnreadBadge:   colorOrDefault(tf.UnreadBadge, d.UnreadBadge),
		PinBadge:      colorOrDefault(tf.PinBadge, d.PinBadge),
		ChannelActive: colorOrDefault(tf.ChannelActive, d.ChannelActive),
		MentionBadge:  colorOrDefault(tf.MentionBadge, d.MentionBadge),
		MentionText:   colorOrDefault(tf.MentionText, d.MentionText),
		MentionSelfBg: colorOrDefault(tf.MentionSelfBg, d.MentionSelfBg),
		TagBadge:      colorOrDefault(tf.TagBadge, d.TagBadge),

		// Left nil unless the file sets them, so derive() can compute them
		// from whatever palette the file actually declared.
		Selection:         hexColor(tf.Selection),
		SearchMatch:       hexColor(tf.SearchMatch),
		SearchMatchActive: hexColor(tf.SearchMatchActive),
	}.derive()
}
