package theme

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

const validThemeTOML = `
name = "Local Test"
author = "tester"
background = "#101010"
foreground = "#f0f0f0"
subtle = "#808080"
accent = "#00ff00"
error = "#ff0000"
success = "#00ff00"
warning = "#ffff00"
border = "#202020"
active_border = "#00ff00"
highlight = "#303030"
muted = "#606060"
username = "#ff00ff"
timestamp = "#808080"
unread_badge = "#00ffff"
pin_badge = "#ffff00"
channel_active = "#00ff00"
mention_badge = "#ff0000"
mention_text = "#00ff00"
mention_self_bg = "#ffff00"
tag_badge = "#00ff00"
`

// writeTheme puts a theme file in a fresh themes dir and returns the dir.
func writeTheme(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	return dir
}

func TestLoadLocal(t *testing.T) {
	dir := writeTheme(t, "mytheme", validThemeTOML)

	th, warnings, err := LoadLocal("mytheme", dir)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("valid theme warned: %v", warnings)
	}
	if th.Name != "Local Test" {
		t.Errorf("Name = %q", th.Name)
	}
	if colorToHex(th.Background) != "#101010" {
		t.Errorf("Background = %s", colorToHex(th.Background))
	}
	if th.Selection == nil {
		t.Error("Selection should have been derived")
	}
}

func TestLoadLocal_NameIsCaseInsensitive(t *testing.T) {
	dir := writeTheme(t, "mytheme", validThemeTOML)

	if _, _, err := LoadLocal("MyTheme", dir); err != nil {
		t.Errorf("LoadLocal(MyTheme): %v", err)
	}
}

func TestLoadLocal_Missing(t *testing.T) {
	_, _, err := LoadLocal("absent", t.TempDir())
	if !errors.Is(err, ErrLocalThemeNotFound) {
		t.Errorf("err = %v, want ErrLocalThemeNotFound", err)
	}
}

func TestLoadLocal_BrokenFileIsAnError(t *testing.T) {
	dir := writeTheme(t, "broken", "name = \"Broken\"\nbackground = \"#101010\"\n")

	_, _, err := LoadLocal("broken", dir)
	if err == nil {
		t.Fatal("expected an error for an incomplete theme")
	}
	if errors.Is(err, ErrLocalThemeNotFound) {
		t.Error("a broken theme must be distinguishable from a missing one")
	}
}

func TestLoadLocal_NamedColors(t *testing.T) {
	body := strings.Replace(validThemeTOML, `accent = "#00ff00"`, `accent = "light_blue"`, 1)
	dir := writeTheme(t, "named", body)

	th, _, err := LoadLocal("named", dir)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	got, ok := th.Accent.(lipgloss.Color)
	if !ok {
		t.Fatalf("Accent = %T, want lipgloss.Color for a named color", th.Accent)
	}
	if string(got) != "12" {
		t.Errorf("Accent = %q, want ANSI 12", string(got))
	}
}

func TestLoadLocal_AuthorDefaults(t *testing.T) {
	body := strings.Replace(validThemeTOML, "author = \"tester\"\n", "", 1)
	dir := writeTheme(t, "noauthor", body)

	th, _, err := LoadLocal("noauthor", dir)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	if th.Author == "" {
		t.Error("Author should fall back rather than render empty")
	}
}

// A theme name reaches this code from a command line or config file and is
// used to build a path, so it must not be able to escape the themes directory.
func TestNormalizeLocalThemeNameRejectsTraversal(t *testing.T) {
	bad := []string{
		"../../etc/passwd",
		"..",
		".",
		"sub/dir",
		`sub\dir`,
		"/etc/passwd",
		"",
		"   ",
	}
	for _, name := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeLocalThemeName(name); err == nil {
				t.Errorf("normalizeLocalThemeName(%q) was accepted", name)
			}
		})
	}
}

func TestNormalizeLocalThemeNameLowercases(t *testing.T) {
	got, err := normalizeLocalThemeName("  MyTheme  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "mytheme" {
		t.Errorf("got %q, want mytheme", got)
	}
}

func TestLoadLocalRejectsTraversal(t *testing.T) {
	if _, _, err := LoadLocal("../escape", t.TempDir()); err == nil {
		t.Error("traversal was accepted")
	}
}

func TestParseThemeColor(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantHex string
		wantAns string
	}{
		{name: "hex", in: "#7aa2f7", wantHex: "#7aa2f7"},
		{name: "shorthand hex", in: "#abc", wantHex: "#aabbcc"},
		{name: "named", in: "blue", wantAns: "4"},
		{name: "named uppercase", in: "BLUE", wantAns: "4"},
		{name: "gray is ansi 7", in: "gray", wantAns: "7"},
		{name: "white is bright", in: "white", wantAns: "15"},
		{name: "unknown name", in: "chartreuse", wantNil: true},
		{name: "empty", in: "", wantNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseThemeColor(tc.in)
			switch {
			case tc.wantNil:
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
			case tc.wantHex != "":
				if colorToHex(got) != tc.wantHex {
					t.Errorf("got %s, want %s", colorToHex(got), tc.wantHex)
				}
			case tc.wantAns != "":
				c, ok := got.(lipgloss.Color)
				if !ok || string(c) != tc.wantAns {
					t.Errorf("got %v, want ANSI %s", got, tc.wantAns)
				}
			}
		})
	}
}

func TestResolveNamed_PrefersBundled(t *testing.T) {
	// A local file named after a bundled theme must not shadow it, or docs
	// referring to "kanagawa" would describe something else.
	dir := writeTheme(t, "kanagawa", strings.Replace(
		validThemeTOML, `background = "#101010"`, `background = "#ffffff"`, 1))

	th, _, err := ResolveNamed("kanagawa", dir)
	if err != nil {
		t.Fatalf("ResolveNamed: %v", err)
	}
	if colorToHex(th.Background) != "#1f1f28" {
		t.Errorf("Background = %s, want the bundled kanagawa", colorToHex(th.Background))
	}
}

func TestResolveNamed_FindsLocal(t *testing.T) {
	dir := writeTheme(t, "custom", validThemeTOML)

	th, _, err := ResolveNamed("custom", dir)
	if err != nil {
		t.Fatalf("ResolveNamed: %v", err)
	}
	if th.Name != "Local Test" {
		t.Errorf("Name = %q", th.Name)
	}
}

func TestResolveNamed_Unknown(t *testing.T) {
	_, _, err := ResolveNamed("nope", t.TempDir())
	if !errors.Is(err, ErrLocalThemeNotFound) {
		t.Errorf("err = %v, want ErrLocalThemeNotFound", err)
	}
}
