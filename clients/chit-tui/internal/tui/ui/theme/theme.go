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

// ListAvailable returns the names a theme can be chosen by: the bundled
// themes, sorted, then the user's TOML themes in themesDir and legacy JSON
// skins, sorted. Each name appears once, in the form ResolveNamed takes, and
// only if it would load: a local file named like a bundled theme is shadowed
// by it, and one whose name is not lowercase is never found, since names
// are lowercased to load.
func ListAvailable(themesDir string) []string {
	names := BuiltinNames()
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}

	var local []string
	for _, src := range []struct{ dir, ext string }{{themesDir, ".toml"}, {skinsDir(), ".json"}} {
		entries, err := os.ReadDir(src.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), src.ext)
			if e.IsDir() || !ok || name != strings.ToLower(name) || seen[name] {
				continue
			}
			if _, err := normalizeLocalThemeName(name); err != nil {
				continue
			}
			seen[name] = true
			local = append(local, name)
		}
	}
	sort.Strings(local)
	return append(names, local...)
}

// LoadNamed resolves a theme by name. It checks built-in themes first,
// then looks for ~/.config/chit/skins/<name>.json.
// Returns the default TokyoNight theme if nothing matches.
func LoadNamed(name string) Theme {
	if name == "" {
		return TokyoNight()
	}
	if t, ok := Lookup(name); ok {
		return t
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
