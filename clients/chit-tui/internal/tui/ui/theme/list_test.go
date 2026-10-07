package theme_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// TOML themes in the themes directory loaded by name, but only the legacy
// JSON skins were listed, so a TOML theme never appeared in /theme or
// --list-themes.
func TestListAvailable_IncludesLocalThemes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	themesDir := filepath.Join(home, "themes")
	skinsDir := filepath.Join(home, ".config", "chit", "skins")
	for _, dir := range []string{themesDir, skinsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		filepath.Join(themesDir, "mine.toml"),
		filepath.Join(themesDir, "tokyo-night.toml"), // shadowed by the bundled theme
		filepath.Join(themesDir, "Shouty.toml"),      // names load lowercased; this one cannot
		filepath.Join(themesDir, "notes.txt"),
		filepath.Join(skinsDir, "old.json"),
		filepath.Join(skinsDir, "mine.json"), // the TOML theme of the same name wins
	} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	names := theme.ListAvailable(themesDir)

	for _, want := range []string{"tokyo-night", "mine", "old"} {
		if n := countOf(names, want); n != 1 {
			t.Errorf("%q listed %d times, want once: %v", want, n, names)
		}
	}
	for _, unwanted := range []string{"Shouty", "shouty", "notes"} {
		if slices.Contains(names, unwanted) {
			t.Errorf("%q listed but cannot load: %v", unwanted, names)
		}
	}
}

func countOf(names []string, want string) int {
	n := 0
	for _, name := range names {
		if name == want {
			n++
		}
	}
	return n
}

func TestRequest_Dark(t *testing.T) {
	light := func() bool { return false }
	tests := []struct {
		name string
		req  theme.Request
		want bool
	}{
		{"flag beats config", theme.Request{FlagAppearance: theme.AppearanceLight, ConfigAppearance: theme.AppearanceDark}, false},
		{"config beats system", theme.Request{ConfigAppearance: theme.AppearanceDark, SystemIsDark: light}, true},
		{"system when unset", theme.Request{SystemIsDark: light}, false},
		{"dark when nothing can tell", theme.Request{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.Dark(); got != tc.want {
				t.Errorf("Dark() = %v, want %v", got, tc.want)
			}
		})
	}
}
