package theme

import (
	"strings"
	"testing"
)

// fixedDark is a DarkDetector with a fixed answer.
type fixedDark bool

func (f fixedDark) IsDark() bool { return bool(f) }

const (
	alwaysDark  = fixedDark(true)
	alwaysLight = fixedDark(false)
)

// countingDark counts how often the terminal would have been queried.
type countingDark struct{ calls *int }

func (c countingDark) IsDark() bool {
	*c.calls++
	return true
}

// Asking the terminal is a side effect, so an explicit appearance must settle
// the question without it, and an unset one must ask exactly once.
func TestDarkAsksTheTerminalOnlyWhenNeeded(t *testing.T) {
	var calls int
	req := Request{FlagAppearance: AppearanceLight, ConfigAppearance: AppearanceDark, System: countingDark{&calls}}
	if req.Dark() {
		t.Error("Dark() = true, want false from the flag")
	}
	req = Request{ConfigAppearance: AppearanceDark, System: countingDark{&calls}}
	if !req.Dark() {
		t.Error("Dark() = false, want true from the config")
	}
	if calls != 0 {
		t.Fatalf("terminal queried %d times with an explicit appearance, want 0", calls)
	}
	req = Request{System: countingDark{&calls}}
	if !req.Dark() {
		t.Error("Dark() = false, want true from the terminal")
	}
	if calls != 1 {
		t.Errorf("terminal queried %d times with no appearance set, want 1", calls)
	}
}

func TestParseAppearance(t *testing.T) {
	tests := []struct {
		in      string
		want    Appearance
		wantErr bool
	}{
		{in: "", want: AppearanceUnset},
		{in: "dark", want: AppearanceDark},
		{in: "light", want: AppearanceLight},
		{in: "system", want: AppearanceSystem},
		{in: "  Dark  ", want: AppearanceDark},
		{in: "DARK", want: AppearanceDark},
		{in: "purple", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseAppearance(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAppearance(%q) accepted", tc.in)
				}
				if !strings.Contains(err.Error(), `"dark", "light", or "system"`) {
					t.Errorf("error does not list the valid values: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAppearance(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAppearanceString(t *testing.T) {
	tests := map[Appearance]string{
		AppearanceUnset:  "",
		AppearanceDark:   "dark",
		AppearanceLight:  "light",
		AppearanceSystem: "system",
	}
	for a, want := range tests {
		if got := a.String(); got != want {
			t.Errorf("Appearance(%d).String() = %q, want %q", a, got, want)
		}
	}
}

// resolvePrecedenceCase is a request and the background its theme must have.
type resolvePrecedenceCase struct {
	name    string
	req     Request
	wantBg  string
	wantErr bool
}

// resolvePrecedenceCases is the precedence matrix: flag beats config theme,
// which beats the appearance-selected pair, which beats the appearance
// default.
func resolvePrecedenceCases() []resolvePrecedenceCase {
	return []resolvePrecedenceCase{
		{
			name: "flag wins over everything",
			req: Request{
				FlagTheme: "nightfox", ConfigTheme: "kanagawa",
				ConfigThemeDark: "catppuccin", System: alwaysDark,
			},
			wantBg: "#192330",
		},
		{
			name: "config theme wins over the appearance pair",
			req: Request{
				ConfigTheme: "kanagawa", ConfigThemeDark: "catppuccin",
				System: alwaysDark,
			},
			wantBg: "#1f1f28",
		},
		{
			name: "theme_dark used when appearance resolves dark",
			req: Request{
				ConfigThemeDark: "catppuccin", ConfigThemeLight: "catppuccin-latte",
				FlagAppearance: AppearanceDark,
			},
			wantBg: "#1e1e2e",
		},
		{
			name: "theme_light used when appearance resolves light",
			req: Request{
				ConfigThemeDark: "catppuccin", ConfigThemeLight: "catppuccin-latte",
				FlagAppearance: AppearanceLight,
			},
			wantBg: "#eff1f5",
		},
		{
			name:   "default dark theme when nothing is configured",
			req:    Request{System: alwaysDark},
			wantBg: "#1a1b26",
		},
		{
			name:   "default light theme when the terminal is light",
			req:    Request{System: alwaysLight},
			wantBg: "#e1e2e7",
		},
		{
			name:   "flag appearance beats config appearance",
			req:    Request{FlagAppearance: AppearanceLight, ConfigAppearance: AppearanceDark},
			wantBg: "#e1e2e7",
		},
		{
			name:   "system detection used when appearance is unset",
			req:    Request{System: alwaysLight},
			wantBg: "#e1e2e7",
		},
		{
			name:    "unknown flag theme is fatal",
			req:     Request{FlagTheme: "no-such-theme"},
			wantErr: true,
		},
	}
}

func TestResolvePrecedence(t *testing.T) {
	for _, tc := range resolvePrecedenceCases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.ThemesDir = t.TempDir()

			got, _, err := Resolve(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if colorToHex(got.Background) != tc.wantBg {
				t.Errorf("Background = %s, want %s", colorToHex(got.Background), tc.wantBg)
			}
		})
	}
}

// A name typed on the command line is worth stopping for; a stale config file
// is not. The error must also say what the valid names are.
func TestResolveUnknownFlagThemeListsBuiltins(t *testing.T) {
	_, _, err := Resolve(Request{FlagTheme: "bogus", ThemesDir: "/tmp/themes"})
	if err == nil {
		t.Fatal("expected an error")
	}

	msg := err.Error()
	for _, want := range []string{"bogus", "tokyo-night", "catppuccin", "/tmp/themes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
}

func TestResolveUnknownConfigThemeWarnsAndFallsThrough(t *testing.T) {
	got, warnings, err := Resolve(Request{
		ConfigTheme: "bogus",
		System:      alwaysDark,
		ThemesDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("an unknown config theme should not be fatal: %v", err)
	}
	if colorToHex(got.Background) != "#1a1b26" {
		t.Errorf("Background = %s, want the default", colorToHex(got.Background))
	}
	if !hasSubstring(warnings, "bogus") {
		t.Errorf("warnings do not mention the bad name: %v", warnings)
	}
}

func TestResolveBrokenConfigThemeWarnsAndFallsThrough(t *testing.T) {
	dir := writeTheme(t, "broken", "name = \"Broken\"\n")

	got, warnings, err := Resolve(Request{
		ConfigTheme: "broken",
		System:      alwaysDark,
		ThemesDir:   dir,
	})
	if err != nil {
		t.Fatalf("a broken config theme should not be fatal: %v", err)
	}
	if colorToHex(got.Background) != "#1a1b26" {
		t.Errorf("Background = %s, want the default", colorToHex(got.Background))
	}
	if !hasSubstring(warnings, "could not be loaded") {
		t.Errorf("warnings do not explain the failure: %v", warnings)
	}
}

// A broken theme named by --theme is fatal, unlike one named in config.
func TestResolveBrokenFlagThemeIsFatal(t *testing.T) {
	dir := writeTheme(t, "broken", "name = \"Broken\"\n")

	if _, _, err := Resolve(Request{FlagTheme: "broken", ThemesDir: dir}); err == nil {
		t.Error("a broken theme named by the flag should be fatal")
	}
}

func TestResolveWarnsWhenAppearanceIsRedundant(t *testing.T) {
	_, warnings, err := Resolve(Request{
		FlagTheme:      "kanagawa",
		FlagAppearance: AppearanceLight,
		ThemesDir:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSubstring(warnings, "appearance is ignored") {
		t.Errorf("expected a warning that appearance is redundant: %v", warnings)
	}
}

func TestResolveLocalThemeByFlag(t *testing.T) {
	dir := writeTheme(t, "custom", validThemeTOML)

	got, _, err := Resolve(Request{FlagTheme: "custom", ThemesDir: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Name != "Local Test" {
		t.Errorf("Name = %q", got.Name)
	}
}

// A nil detector must not panic; dark is the safer assumption for a terminal.
func TestResolveNilSystemDetectorDefaultsToDark(t *testing.T) {
	got, _, err := Resolve(Request{ThemesDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if colorToHex(got.Background) != "#1a1b26" {
		t.Errorf("Background = %s, want the dark default", colorToHex(got.Background))
	}
}

// theme_light must be ignored on a dark terminal, and vice versa — otherwise
// configuring both would be pointless.
func TestResolveIgnoresTheWrongAppearancePair(t *testing.T) {
	req := Request{
		ConfigThemeLight: "catppuccin-latte",
		FlagAppearance:   AppearanceDark,
		ThemesDir:        t.TempDir(),
	}

	got, _, err := Resolve(req)
	if err != nil {
		t.Fatal(err)
	}
	if colorToHex(got.Background) != "#1a1b26" {
		t.Errorf("Background = %s; theme_light should be ignored when dark",
			colorToHex(got.Background))
	}
}

func TestResolveUnknownAppearancePairWarns(t *testing.T) {
	_, warnings, err := Resolve(Request{
		ConfigThemeDark: "bogus",
		FlagAppearance:  AppearanceDark,
		ThemesDir:       t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSubstring(warnings, "bogus") {
		t.Errorf("warnings do not mention the bad name: %v", warnings)
	}
}

func hasSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
