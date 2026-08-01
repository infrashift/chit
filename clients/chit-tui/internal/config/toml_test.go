package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

// writeFile writes content to a temp file and returns its path.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadFile_Absent(t *testing.T) {
	settings, warnings := config.LoadFile(filepath.Join(t.TempDir(), "nope.toml"))
	if len(warnings) != 0 {
		t.Errorf("a missing config file is normal and should not warn: %v", warnings)
	}
	if settings.Theme != "" {
		t.Errorf("Theme = %q, want empty", settings.Theme)
	}
}

func TestLoadFile_ReadsSettings(t *testing.T) {
	path := writeFile(t, "config.toml", `
server_url = "http://example.test:8065"
ws_scheme = "wss"
theme = "catppuccin-latte"
theme_dark = "nightfox"
theme_light = "dayfox"
appearance = "system"
`)

	settings, warnings := config.LoadFile(path)
	if len(warnings) != 0 {
		t.Fatalf("valid config warned: %v", warnings)
	}

	tests := []struct{ name, got, want string }{
		{"ServerURL", settings.ServerURL, "http://example.test:8065"},
		{"WSScheme", settings.WSScheme, "wss"},
		{"Theme", settings.Theme, "catppuccin-latte"},
		{"ThemeDark", settings.ThemeDark, "nightfox"},
		{"ThemeLight", settings.ThemeLight, "dayfox"},
		{"Appearance", settings.Appearance, "system"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// A broken config file must never stop the client from starting — the whole
// point of the forgiving pipeline is that a typo costs you one setting, not
// your chat client.
func TestLoadFile_InvalidTOMLFallsBackToDefaults(t *testing.T) {
	path := writeFile(t, "config.toml", "this is not = valid toml [[[")

	settings, warnings := config.LoadFile(path)
	if len(warnings) == 0 {
		t.Error("invalid TOML should warn")
	}
	if settings.Theme != "" {
		t.Errorf("Theme = %q, want empty", settings.Theme)
	}
}

func TestLoadFile_UnknownKeyIsDroppedWithWarning(t *testing.T) {
	path := writeFile(t, "config.toml", `
theme = "kanagawa"
notasetting = "x"
`)

	settings, warnings := config.LoadFile(path)
	if settings.Theme != "kanagawa" {
		t.Errorf("Theme = %q; a bad neighbor key should not affect it", settings.Theme)
	}
	if !containsSubstring(warnings, "notasetting") {
		t.Errorf("warnings do not mention the unknown key: %v", warnings)
	}
}

// The delete-and-revalidate loop exists so one bad key does not take the whole
// document down with it.
func TestLoadFile_BadValueDropsOnlyThatKey(t *testing.T) {
	path := writeFile(t, "config.toml", `
theme = "kanagawa"
appearance = "purple"
`)

	settings, warnings := config.LoadFile(path)
	if settings.Appearance != "" {
		t.Errorf("Appearance = %q, want it dropped", settings.Appearance)
	}
	if settings.Theme != "kanagawa" {
		t.Errorf("Theme = %q, want it kept", settings.Theme)
	}
	if !containsSubstring(warnings, "appearance") {
		t.Errorf("warnings do not mention appearance: %v", warnings)
	}
}

func TestLoadFile_SeveralBadValuesAreAllReported(t *testing.T) {
	path := writeFile(t, "config.toml", `
theme = "kanagawa"
appearance = "purple"
ws_scheme = "carrier-pigeon"
`)

	settings, warnings := config.LoadFile(path)
	if settings.Theme != "kanagawa" {
		t.Errorf("Theme = %q, want it kept", settings.Theme)
	}
	for _, key := range []string{"appearance", "ws_scheme"} {
		if !containsSubstring(warnings, key) {
			t.Errorf("warnings do not mention %s: %v", key, warnings)
		}
	}
}

// CUE's own diagnostics name internal paths and constraint syntax. Users edit
// TOML, so they must never see that vocabulary.
func TestLoadFile_WarningsAreHumanReadable(t *testing.T) {
	path := writeFile(t, "config.toml", `appearance = "purple"`)

	_, warnings := config.LoadFile(path)
	if len(warnings) == 0 {
		t.Fatal("expected a warning")
	}
	for _, w := range warnings {
		for _, leak := range []string{"#Config", "conflicting values", "cue:"} {
			if strings.Contains(w, leak) {
				t.Errorf("warning leaks CUE internals (%q): %s", leak, w)
			}
		}
	}
	if !containsSubstring(warnings, `"dark", "light", or "system"`) {
		t.Errorf("warning does not explain the allowed values: %v", warnings)
	}
}

func TestLoadFile_WrongTypeIsDropped(t *testing.T) {
	path := writeFile(t, "config.toml", `
theme = 42
server_url = "http://example.test"
`)

	settings, warnings := config.LoadFile(path)
	if settings.Theme != "" {
		t.Errorf("Theme = %q, want dropped", settings.Theme)
	}
	if settings.ServerURL != "http://example.test" {
		t.Errorf("ServerURL = %q, want it kept", settings.ServerURL)
	}
	if !containsSubstring(warnings, "theme") {
		t.Errorf("warnings do not mention theme: %v", warnings)
	}
}

// Environment beats the file: the file is the persistent choice, an env var is
// a deliberate override for this invocation.
func TestLoad_EnvBeatsFile(t *testing.T) {
	path := writeFile(t, "config.toml", `
server_url = "http://from-file.test"
theme = "from-file-theme"
`)
	t.Setenv("CHIT_CONFIG_FILE", path)
	t.Setenv("CHIT_SERVER_URL", "http://from-env.test")
	t.Setenv("CHIT_THEME", "from-env-theme")

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "http://from-env.test" {
		t.Errorf("ServerURL = %q, want the env value", cfg.ServerURL)
	}
	if cfg.ThemeName != "from-env-theme" {
		t.Errorf("ThemeName = %q, want the env value", cfg.ThemeName)
	}
}

func TestLoad_FileUsedWhenEnvUnset(t *testing.T) {
	path := writeFile(t, "config.toml", `
server_url = "http://from-file.test"
theme = "kanagawa-lotus"
appearance = "light"
`)
	t.Setenv("CHIT_CONFIG_FILE", path)
	t.Setenv("CHIT_SERVER_URL", "")
	t.Setenv("CHIT_THEME", "")

	cfg, warnings, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v (warnings %v)", err, warnings)
	}
	if cfg.ServerURL != "http://from-file.test" {
		t.Errorf("ServerURL = %q, want the file value", cfg.ServerURL)
	}
	if cfg.ThemeName != "kanagawa-lotus" {
		t.Errorf("ThemeName = %q, want the file value", cfg.ThemeName)
	}
	if cfg.Appearance != "light" {
		t.Errorf("Appearance = %q, want light", cfg.Appearance)
	}
}

// A server_url in the config file satisfies the requirement that used to be
// environment-only, so a user can configure the client once and forget it.
func TestLoad_FileSatisfiesRequiredServerURL(t *testing.T) {
	path := writeFile(t, "config.toml", `server_url = "http://only-in-file.test"`)
	t.Setenv("CHIT_CONFIG_FILE", path)
	t.Setenv("CHIT_SERVER_URL", "")

	if _, _, err := config.Load(); err != nil {
		t.Fatalf("server_url from the file should satisfy validation: %v", err)
	}
}

func TestFilePathHonoursOverride(t *testing.T) {
	t.Setenv("CHIT_CONFIG_FILE", "/tmp/somewhere/else.toml")
	if got := config.FilePath(); got != "/tmp/somewhere/else.toml" {
		t.Errorf("FilePath() = %q", got)
	}
}

func TestDirAndThemesDir(t *testing.T) {
	if !strings.HasSuffix(config.Dir(), "chit") {
		t.Errorf("Dir() = %q, want it to end in chit", config.Dir())
	}
	if filepath.Base(config.ThemesDir()) != "themes" {
		t.Errorf("ThemesDir() = %q", config.ThemesDir())
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
