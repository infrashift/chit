package theme

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLegacySkin points XDG_CONFIG_HOME at a temp dir and writes a JSON skin
// into it, so the pre-TOML lookup path can be exercised without touching the
// developer's real ~/.config.
func writeLegacySkin(t *testing.T, name, body string) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	dir := filepath.Join(root, "chit", "skins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skins: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write skin: %v", err)
	}
}

// Someone upgrading has skins in the old JSON directory. They keep working,
// with a warning pointing at the new location — a silent break would look like
// the theme simply vanished.
func TestResolveNamedFallsBackToLegacySkin(t *testing.T) {
	writeLegacySkin(t, "myskin", `{"name":"My Skin","background":"#123456"}`)

	got, warnings, err := ResolveNamed("myskin", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNamed: %v", err)
	}
	if colorToHex(got.Background) != "#123456" {
		t.Errorf("Background = %s, want #123456", colorToHex(got.Background))
	}
	if !hasSubstring(warnings, "legacy skins directory") {
		t.Errorf("warnings should point at the new location: %v", warnings)
	}
}

// A TOML theme and a legacy skin of the same name: the TOML one wins, since it
// is the format the user has already migrated to.
func TestTOMLThemeBeatsLegacySkin(t *testing.T) {
	writeLegacySkin(t, "dup", `{"name":"Legacy","background":"#111111"}`)
	dir := writeTheme(t, "dup", validThemeTOML)

	got, _, err := ResolveNamed("dup", dir)
	if err != nil {
		t.Fatalf("ResolveNamed: %v", err)
	}
	if colorToHex(got.Background) != "#101010" {
		t.Errorf("Background = %s, want the TOML theme", colorToHex(got.Background))
	}
}

func TestLegacySkinNotFound(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	if _, ok := loadLegacySkin("absent"); ok {
		t.Error("loadLegacySkin reported a missing skin as found")
	}
}

func TestLegacySkinRejectsTraversal(t *testing.T) {
	if _, ok := loadLegacySkin("../escape"); ok {
		t.Error("loadLegacySkin accepted a traversing name")
	}
}

func TestListAvailableIncludesLegacySkins(t *testing.T) {
	writeLegacySkin(t, "myskin", `{"name":"My Skin"}`)

	names := ListAvailable(t.TempDir())

	var sawSkin, sawBuiltin bool
	for _, n := range names {
		switch n {
		case "myskin":
			sawSkin = true
		case "tokyo-night":
			sawBuiltin = true
		}
	}
	if !sawSkin {
		t.Errorf("ListAvailable omitted the legacy skin: %v", names)
	}
	if !sawBuiltin {
		t.Errorf("ListAvailable omitted the bundled themes: %v", names)
	}
}

func TestSkinsDirFallsBackWithoutHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/nobody")

	if got := skinsDir(); got == "" {
		t.Error("skinsDir returned empty")
	}
}
