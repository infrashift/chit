package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines all colors used throughout the TUI.
type Theme struct {
	Name          string
	Author        string
	Background    lipgloss.Color
	Foreground    lipgloss.Color
	Subtle        lipgloss.Color
	Accent        lipgloss.Color
	Error         lipgloss.Color
	Success       lipgloss.Color
	Warning       lipgloss.Color
	Border        lipgloss.Color
	ActiveBorder  lipgloss.Color
	Highlight     lipgloss.Color
	Muted         lipgloss.Color
	Username      lipgloss.Color
	Timestamp     lipgloss.Color
	UnreadBadge   lipgloss.Color
	PinBadge      lipgloss.Color
	ChannelActive lipgloss.Color
	MentionBadge  lipgloss.Color
	MentionText   lipgloss.Color
	MentionSelfBg lipgloss.Color
	TagBadge      lipgloss.Color
}

// themeFile is the JSON representation of a theme file.
type themeFile struct {
	Name          string `json:"name"`
	Author        string `json:"author"`
	Background    string `json:"background"`
	Foreground    string `json:"foreground"`
	Subtle        string `json:"subtle"`
	Accent        string `json:"accent"`
	Error         string `json:"error"`
	Success       string `json:"success"`
	Warning       string `json:"warning"`
	Border        string `json:"border"`
	ActiveBorder  string `json:"active_border"`
	Highlight     string `json:"highlight"`
	Muted         string `json:"muted"`
	Username      string `json:"username"`
	Timestamp     string `json:"timestamp"`
	UnreadBadge   string `json:"unread_badge"`
	PinBadge      string `json:"pin_badge"`
	ChannelActive string `json:"channel_active"`
	MentionBadge  string `json:"mention_badge"`
	MentionText   string `json:"mention_text"`
	MentionSelfBg string `json:"mention_self_bg"`
	TagBadge      string `json:"tag_badge"`
}

// TokyoNight returns the default Tokyo Night theme.
func TokyoNight() Theme {
	return Theme{
		Name:          "Tokyo Night",
		Author:        "chit-tui",
		Background:    lipgloss.Color("#1a1b26"),
		Foreground:    lipgloss.Color("#c0caf5"),
		Subtle:        lipgloss.Color("#565f89"),
		Accent:        lipgloss.Color("#7aa2f7"),
		Error:         lipgloss.Color("#f7768e"),
		Success:       lipgloss.Color("#9ece6a"),
		Warning:       lipgloss.Color("#e0af68"),
		Border:        lipgloss.Color("#3b4261"),
		ActiveBorder:  lipgloss.Color("#7aa2f7"),
		Highlight:     lipgloss.Color("#292e42"),
		Muted:         lipgloss.Color("#545c7e"),
		Username:      lipgloss.Color("#bb9af7"),
		Timestamp:     lipgloss.Color("#565f89"),
		UnreadBadge:   lipgloss.Color("#7aa2f7"),
		PinBadge:      lipgloss.Color("#e0af68"),
		ChannelActive: lipgloss.Color("#7aa2f7"),
		MentionBadge:  lipgloss.Color("#f7768e"),
		MentionText:   lipgloss.Color("#7aa2f7"),
		MentionSelfBg: lipgloss.Color("#e0af68"),
		TagBadge:      lipgloss.Color("#9ece6a"),
	}
}

// Catppuccin returns the Catppuccin Mocha theme.
func Catppuccin() Theme {
	return Theme{
		Name:          "Catppuccin Mocha",
		Author:        "catppuccin",
		Background:    lipgloss.Color("#1e1e2e"),
		Foreground:    lipgloss.Color("#cdd6f4"),
		Subtle:        lipgloss.Color("#6c7086"),
		Accent:        lipgloss.Color("#89b4fa"),
		Error:         lipgloss.Color("#f38ba8"),
		Success:       lipgloss.Color("#a6e3a1"),
		Warning:       lipgloss.Color("#f9e2af"),
		Border:        lipgloss.Color("#313244"),
		ActiveBorder:  lipgloss.Color("#89b4fa"),
		Highlight:     lipgloss.Color("#313244"),
		Muted:         lipgloss.Color("#585b70"),
		Username:      lipgloss.Color("#cba6f7"),
		Timestamp:     lipgloss.Color("#6c7086"),
		UnreadBadge:   lipgloss.Color("#89b4fa"),
		PinBadge:      lipgloss.Color("#f9e2af"),
		ChannelActive: lipgloss.Color("#89b4fa"),
		MentionBadge:  lipgloss.Color("#f38ba8"),
		MentionText:   lipgloss.Color("#89b4fa"),
		MentionSelfBg: lipgloss.Color("#f9e2af"),
		TagBadge:      lipgloss.Color("#a6e3a1"),
	}
}

// Kanagawa returns the Kanagawa theme.
func Kanagawa() Theme {
	return Theme{
		Name:          "Kanagawa",
		Author:        "rebelot",
		Background:    lipgloss.Color("#1f1f28"),
		Foreground:    lipgloss.Color("#dcd7ba"),
		Subtle:        lipgloss.Color("#727169"),
		Accent:        lipgloss.Color("#7e9cd8"),
		Error:         lipgloss.Color("#e82424"),
		Success:       lipgloss.Color("#98bb6c"),
		Warning:       lipgloss.Color("#e6c384"),
		Border:        lipgloss.Color("#2a2a37"),
		ActiveBorder:  lipgloss.Color("#7e9cd8"),
		Highlight:     lipgloss.Color("#2a2a37"),
		Muted:         lipgloss.Color("#54546d"),
		Username:      lipgloss.Color("#957fb8"),
		Timestamp:     lipgloss.Color("#727169"),
		UnreadBadge:   lipgloss.Color("#7fb4ca"),
		PinBadge:      lipgloss.Color("#e6c384"),
		ChannelActive: lipgloss.Color("#7e9cd8"),
		MentionBadge:  lipgloss.Color("#e82424"),
		MentionText:   lipgloss.Color("#7e9cd8"),
		MentionSelfBg: lipgloss.Color("#e6c384"),
		TagBadge:      lipgloss.Color("#98bb6c"),
	}
}

// Nightfox returns the Nightfox theme.
func Nightfox() Theme {
	return Theme{
		Name:          "Nightfox",
		Author:        "EdenEast",
		Background:    lipgloss.Color("#192330"),
		Foreground:    lipgloss.Color("#cdcecf"),
		Subtle:        lipgloss.Color("#71839b"),
		Accent:        lipgloss.Color("#719cd6"),
		Error:         lipgloss.Color("#c94f6d"),
		Success:       lipgloss.Color("#81b29a"),
		Warning:       lipgloss.Color("#dbc074"),
		Border:        lipgloss.Color("#29394f"),
		ActiveBorder:  lipgloss.Color("#719cd6"),
		Highlight:     lipgloss.Color("#29394f"),
		Muted:         lipgloss.Color("#575860"),
		Username:      lipgloss.Color("#9d79d6"),
		Timestamp:     lipgloss.Color("#71839b"),
		UnreadBadge:   lipgloss.Color("#63cdcf"),
		PinBadge:      lipgloss.Color("#dbc074"),
		ChannelActive: lipgloss.Color("#719cd6"),
		MentionBadge:  lipgloss.Color("#c94f6d"),
		MentionText:   lipgloss.Color("#719cd6"),
		MentionSelfBg: lipgloss.Color("#dbc074"),
		TagBadge:      lipgloss.Color("#81b29a"),
	}
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

func colorOrDefault(val string, def lipgloss.Color) lipgloss.Color {
	if val == "" {
		return def
	}
	return lipgloss.Color(val)
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
	}
}
