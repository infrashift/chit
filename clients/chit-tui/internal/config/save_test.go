package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

func TestSaveTheme_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("kanagawa"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	settings, warnings := config.LoadFile(path)
	if len(warnings) != 0 {
		t.Errorf("saved config does not load cleanly: %v", warnings)
	}
	if settings.Theme != "kanagawa" {
		t.Errorf("Theme = %q, want kanagawa", settings.Theme)
	}
}

func TestSaveTheme_ReplacesExistingKey(t *testing.T) {
	path := writeFile(t, "config.toml", "theme = \"nightfox\"\nserver_url = \"http://x.test\"\n")
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("dayfox"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	settings, _ := config.LoadFile(path)
	if settings.Theme != "dayfox" {
		t.Errorf("Theme = %q, want dayfox", settings.Theme)
	}
	if settings.ServerURL != "http://x.test" {
		t.Errorf("ServerURL = %q; saving a theme must not disturb other settings", settings.ServerURL)
	}

	// One assignment, not two.
	body, _ := os.ReadFile(path)
	if n := strings.Count(string(body), "theme ="); n != 1 {
		t.Errorf("file has %d theme assignments, want 1:\n%s", n, body)
	}
}

// A round-trip through the TOML encoder would drop comments, so the file is
// edited line by line instead. This is the test that pins that decision.
func TestSaveTheme_PreservesComments(t *testing.T) {
	original := `# My chit configuration
# Keep this comment.

server_url = "http://x.test"  # inline note
theme = "nightfox"

# Trailing note.
`
	path := writeFile(t, "config.toml", original)
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("catppuccin"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	for _, want := range []string{
		"# My chit configuration",
		"# Keep this comment.",
		"# inline note",
		"# Trailing note.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("comment %q was lost:\n%s", want, got)
		}
	}
	if !strings.Contains(got, `theme = "catppuccin"`) {
		t.Errorf("theme was not updated:\n%s", got)
	}
}

func TestSaveTheme_AppendsWhenAbsent(t *testing.T) {
	path := writeFile(t, "config.toml", "server_url = \"http://x.test\"\n")
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("dayfox"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	settings, warnings := config.LoadFile(path)
	if len(warnings) != 0 {
		t.Errorf("appended config does not load cleanly: %v", warnings)
	}
	if settings.Theme != "dayfox" {
		t.Errorf("Theme = %q", settings.Theme)
	}
	if settings.ServerURL != "http://x.test" {
		t.Errorf("ServerURL = %q", settings.ServerURL)
	}
}

// A file with no trailing newline must not have the new key glued onto the
// last line.
func TestSaveTheme_HandlesMissingTrailingNewline(t *testing.T) {
	path := writeFile(t, "config.toml", `server_url = "http://x.test"`)
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("dayfox"); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	settings, warnings := config.LoadFile(path)
	if len(warnings) != 0 {
		t.Errorf("config does not load cleanly: %v", warnings)
	}
	if settings.Theme != "dayfox" || settings.ServerURL != "http://x.test" {
		body, _ := os.ReadFile(path)
		t.Errorf("Theme = %q, ServerURL = %q; file:\n%s",
			settings.Theme, settings.ServerURL, body)
	}
}

func TestSaveTheme_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("CHIT_CONFIG_FILE", path)

	for _, name := range []string{"kanagawa", "dayfox", "catppuccin-latte"} {
		if err := config.SaveTheme(name); err != nil {
			t.Fatalf("SaveTheme(%s): %v", name, err)
		}
		settings, warnings := config.LoadFile(path)
		if len(warnings) != 0 {
			t.Errorf("after saving %s: %v", name, warnings)
		}
		if settings.Theme != name {
			t.Errorf("Theme = %q, want %q", settings.Theme, name)
		}
	}
}

// Names are quoted, so one containing a quote cannot break out and corrupt the
// file into something that no longer parses.
func TestSaveTheme_QuotesAwkwardNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme(`we"ird`); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	settings, warnings := config.LoadFile(path)
	if len(warnings) != 0 {
		t.Errorf("file no longer parses: %v", warnings)
	}
	if settings.Theme != `we"ird` {
		t.Errorf("Theme = %q", settings.Theme)
	}
}

// A config directory that cannot be written must surface an error rather than
// failing silently — the caller reports it while keeping the theme applied
// for the current session.
func TestSaveTheme_UnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	dir := t.TempDir()
	readonly := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readonly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })

	t.Setenv("CHIT_CONFIG_FILE", filepath.Join(readonly, "config.toml"))

	if err := config.SaveTheme("kanagawa"); err == nil {
		t.Error("expected an error when the config directory is not writable")
	}
}

// An unreadable existing config must not be silently replaced: that would
// discard settings the user cannot see.
func TestSaveTheme_UnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}

	path := writeFile(t, "config.toml", "theme = \"nightfox\"\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	t.Setenv("CHIT_CONFIG_FILE", path)

	if err := config.SaveTheme("kanagawa"); err == nil {
		t.Error("expected an error when the existing config cannot be read")
	}
}

// The picker always wrote "theme", which outranks theme_dark and
// theme_light, so one pick turned off appearance switching for good.
func TestSaveThemeSetting_WritesOnlyItsKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("CHIT_CONFIG_FILE", path)
	if err := os.WriteFile(path, []byte("theme_dark = \"kanagawa\"\ntheme_light = \"dayfox\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.SaveThemeSetting("theme_dark", "nightfox"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if want := "theme_dark = \"nightfox\"\ntheme_light = \"dayfox\"\n"; string(got) != want {
		t.Errorf("config = %q, want %q", got, want)
	}
}

func TestSaveThemeSetting_RejectsOtherKeys(t *testing.T) {
	t.Setenv("CHIT_CONFIG_FILE", filepath.Join(t.TempDir(), "config.toml"))
	if err := config.SaveThemeSetting("server_url", "x"); err == nil {
		t.Error("wrote a setting that is not a theme")
	}
}

func TestConfig_ThemeSettingKey(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		dark bool
		want string
	}{
		{"no themes configured", config.Config{}, true, "theme"},
		{"a fixed theme", config.Config{ThemeName: "kanagawa", ThemeDark: "nightfox"}, true, "theme"},
		{"per appearance, dark", config.Config{ThemeDark: "nightfox"}, true, "theme_dark"},
		{"per appearance, light", config.Config{ThemeLight: "dayfox"}, false, "theme_light"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.ThemeSettingKey(tc.dark); got != tc.want {
				t.Errorf("ThemeSettingKey(%v) = %q, want %q", tc.dark, got, tc.want)
			}
		})
	}
}
