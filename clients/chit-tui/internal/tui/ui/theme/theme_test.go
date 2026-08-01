package theme_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func TestTokyoNight_HasAllColors(t *testing.T) {
	tn := theme.TokyoNight()

	checks := []struct {
		name  string
		color string
	}{
		{"Background", theme.Hex(tn.Background)},
		{"Foreground", theme.Hex(tn.Foreground)},
		{"Subtle", theme.Hex(tn.Subtle)},
		{"Accent", theme.Hex(tn.Accent)},
		{"Error", theme.Hex(tn.Error)},
		{"Success", theme.Hex(tn.Success)},
		{"Warning", theme.Hex(tn.Warning)},
		{"Border", theme.Hex(tn.Border)},
		{"ActiveBorder", theme.Hex(tn.ActiveBorder)},
		{"Highlight", theme.Hex(tn.Highlight)},
		{"Muted", theme.Hex(tn.Muted)},
		{"Username", theme.Hex(tn.Username)},
		{"Timestamp", theme.Hex(tn.Timestamp)},
		{"UnreadBadge", theme.Hex(tn.UnreadBadge)},
		{"PinBadge", theme.Hex(tn.PinBadge)},
		{"ChannelActive", theme.Hex(tn.ChannelActive)},
		{"MentionBadge", theme.Hex(tn.MentionBadge)},
		{"MentionText", theme.Hex(tn.MentionText)},
		{"MentionSelfBg", theme.Hex(tn.MentionSelfBg)},
	}

	for _, c := range checks {
		if c.color == "" {
			t.Errorf("Theme.%s is empty", c.name)
		}
	}
}

func TestTokyoNight_HasNameAndAuthor(t *testing.T) {
	tn := theme.TokyoNight()
	if tn.Name == "" {
		t.Error("expected Name to be set")
	}
	if tn.Author == "" {
		t.Error("expected Author to be set")
	}
}

func TestParseJSON_FullTheme(t *testing.T) {
	data := []byte(`{
		"name": "Dracula",
		"author": "test",
		"background": "#282a36",
		"foreground": "#f8f8f2",
		"subtle": "#6272a4",
		"accent": "#bd93f9",
		"error": "#ff5555",
		"success": "#50fa7b",
		"warning": "#f1fa8c",
		"border": "#44475a",
		"active_border": "#bd93f9",
		"highlight": "#44475a",
		"muted": "#6272a4",
		"username": "#ff79c6",
		"timestamp": "#6272a4",
		"unread_badge": "#8be9fd",
		"pin_badge": "#f1fa8c",
		"channel_active": "#bd93f9"
	}`)

	th, err := theme.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "Dracula" {
		t.Errorf("name = %q, want %q", th.Name, "Dracula")
	}
	if theme.Hex(th.Background) != "#282a36" {
		t.Errorf("background = %q", theme.Hex(th.Background))
	}
	if theme.Hex(th.Error) != "#ff5555" {
		t.Errorf("error = %q", theme.Hex(th.Error))
	}
}

func TestParseJSON_PartialTheme_DefaultsFilled(t *testing.T) {
	data := []byte(`{"name": "Partial", "background": "#000000"}`)

	th, err := theme.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "Partial" {
		t.Errorf("name = %q", th.Name)
	}
	if theme.Hex(th.Background) != "#000000" {
		t.Errorf("background = %q", theme.Hex(th.Background))
	}
	// Unspecified fields should fall back to TokyoNight defaults
	def := theme.TokyoNight()
	if th.Foreground != def.Foreground {
		t.Errorf("foreground = %q, want default %q", theme.Hex(th.Foreground), theme.Hex(def.Foreground))
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	_, err := theme.ParseJSON([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{"name": "TestTheme", "accent": "#ff0000"}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	th, err := theme.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.Name != "TestTheme" {
		t.Errorf("name = %q", th.Name)
	}
	if theme.Hex(th.Accent) != "#ff0000" {
		t.Errorf("accent = %q", theme.Hex(th.Accent))
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := theme.LoadFromFile("/nonexistent/theme.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadNamed_Empty_ReturnsDefault(t *testing.T) {
	th := theme.LoadNamed("")
	def := theme.TokyoNight()
	if th.Name != def.Name {
		t.Errorf("name = %q, want %q", th.Name, def.Name)
	}
}

func TestLoadNamed_Missing_ReturnsDefault(t *testing.T) {
	th := theme.LoadNamed("nonexistent-theme-name-xyz")
	def := theme.TokyoNight()
	if th.Name != def.Name {
		t.Errorf("name = %q, want default %q", th.Name, def.Name)
	}
}

func assertThemeComplete(t *testing.T, th theme.Theme) {
	t.Helper()
	if th.Name == "" {
		t.Error("Name is empty")
	}
	if th.Author == "" {
		t.Error("Author is empty")
	}
	colors := []struct {
		name  string
		color string
	}{
		{"Background", theme.Hex(th.Background)},
		{"Foreground", theme.Hex(th.Foreground)},
		{"Subtle", theme.Hex(th.Subtle)},
		{"Accent", theme.Hex(th.Accent)},
		{"Error", theme.Hex(th.Error)},
		{"Success", theme.Hex(th.Success)},
		{"Warning", theme.Hex(th.Warning)},
		{"Border", theme.Hex(th.Border)},
		{"ActiveBorder", theme.Hex(th.ActiveBorder)},
		{"Highlight", theme.Hex(th.Highlight)},
		{"Muted", theme.Hex(th.Muted)},
		{"Username", theme.Hex(th.Username)},
		{"Timestamp", theme.Hex(th.Timestamp)},
		{"UnreadBadge", theme.Hex(th.UnreadBadge)},
		{"PinBadge", theme.Hex(th.PinBadge)},
		{"ChannelActive", theme.Hex(th.ChannelActive)},
		{"MentionBadge", theme.Hex(th.MentionBadge)},
		{"MentionText", theme.Hex(th.MentionText)},
		{"MentionSelfBg", theme.Hex(th.MentionSelfBg)},
	}
	for _, c := range colors {
		if c.color == "" {
			t.Errorf("%s is empty", c.name)
		}
	}
}

func TestCatppuccin_HasAllColors(t *testing.T) {
	th := theme.Catppuccin()
	assertThemeComplete(t, th)
	if th.Name != "Catppuccin Mocha" {
		t.Errorf("name = %q", th.Name)
	}
}

func TestKanagawa_HasAllColors(t *testing.T) {
	th := theme.Kanagawa()
	assertThemeComplete(t, th)
	if th.Name != "Kanagawa" {
		t.Errorf("name = %q", th.Name)
	}
}

func TestNightfox_HasAllColors(t *testing.T) {
	th := theme.Nightfox()
	assertThemeComplete(t, th)
	if th.Name != "Nightfox" {
		t.Errorf("name = %q", th.Name)
	}
}

func TestLoadNamed_BuiltinCatppuccin(t *testing.T) {
	th := theme.LoadNamed("catppuccin")
	if th.Name != "Catppuccin Mocha" {
		t.Errorf("name = %q, want %q", th.Name, "Catppuccin Mocha")
	}
}

func TestLoadNamed_BuiltinKanagawa(t *testing.T) {
	th := theme.LoadNamed("kanagawa")
	if th.Name != "Kanagawa" {
		t.Errorf("name = %q, want %q", th.Name, "Kanagawa")
	}
}

func TestLoadNamed_BuiltinNightfox(t *testing.T) {
	th := theme.LoadNamed("nightfox")
	if th.Name != "Nightfox" {
		t.Errorf("name = %q, want %q", th.Name, "Nightfox")
	}
}

func TestLoadNamed_BuiltinTokyoNight(t *testing.T) {
	th := theme.LoadNamed("tokyo-night")
	if th.Name != "Tokyo Night" {
		t.Errorf("name = %q, want %q", th.Name, "Tokyo Night")
	}
}

func TestListAvailable_IncludesBuiltins(t *testing.T) {
	names := theme.ListAvailable()

	builtins := []string{"catppuccin", "kanagawa", "nightfox", "tokyo-night"}
	for _, b := range builtins {
		found := false
		for _, n := range names {
			if n == b {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in ListAvailable, got %v", b, names)
		}
	}

	// Verify built-ins are sorted
	for i := 1; i < len(builtins); i++ {
		idx1, idx2 := -1, -1
		for j, n := range names {
			if n == builtins[i-1] {
				idx1 = j
			}
			if n == builtins[i] {
				idx2 = j
			}
		}
		if idx1 >= idx2 {
			t.Errorf("expected %q before %q in sorted list", builtins[i-1], builtins[i])
		}
	}
}

func TestListAvailable_IncludesCustomSkins(t *testing.T) {
	// Set XDG_CONFIG_HOME to a temp dir
	dir := t.TempDir()
	skinsDir := filepath.Join(dir, "chit", "skins")
	if err := os.MkdirAll(skinsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skinsDir, "dracula.json"), []byte(`{"name":"Dracula"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)

	names := theme.ListAvailable()
	found := false
	for _, n := range names {
		if n == "dracula" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'dracula' in ListAvailable, got %v", names)
	}
}
