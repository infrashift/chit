package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     options
		wantErr  bool
		wantHelp bool
	}{
		{name: "no flags", args: nil, want: options{}},
		{name: "theme", args: []string{"--theme", "kanagawa"},
			want: options{theme: "kanagawa"}},
		{name: "theme with equals", args: []string{"--theme=dayfox"},
			want: options{theme: "dayfox"}},
		{name: "single dash", args: []string{"-theme", "nightfox"},
			want: options{theme: "nightfox"}},
		{name: "appearance", args: []string{"--appearance", "light"},
			want: options{appearance: "light"}},
		{name: "both", args: []string{"--theme", "x", "--appearance", "dark"},
			want: options{theme: "x", appearance: "dark"}},
		{name: "list themes", args: []string{"--list-themes"},
			want: options{listThemes: true}},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
		{name: "help", args: []string{"--help"}, wantErr: true, wantHelp: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			got, err := parseFlags(&out, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseFlags(%v) accepted", tc.args)
				}
				// The flag package renders the single-dash form; both spellings
				// are accepted on the command line.
				if tc.wantHelp && !strings.Contains(out.String(), "-theme string") {
					t.Errorf("help output does not document --theme:\n%s", out.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFlags(%v): %v", tc.args, err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Usage names the config file so someone who has never read the docs can find
// out where settings live.
func TestParseFlagsUsageMentionsConfigFile(t *testing.T) {
	var out bytes.Buffer
	_, _ = parseFlags(&out, []string{"--help"})

	if !strings.Contains(out.String(), config.FilePath()) {
		t.Errorf("usage does not mention the config file path:\n%s", out.String())
	}
}

func TestListThemes(t *testing.T) {
	var out bytes.Buffer
	listThemes(&out)

	got := out.String()
	for _, name := range []string{"tokyo-night", "catppuccin-latte", "dayfox"} {
		if !strings.Contains(got, name) {
			t.Errorf("listThemes omitted %q:\n%s", name, got)
		}
	}
}

func TestResolveTheme_FlagWins(t *testing.T) {
	cfg := &config.Config{ThemeName: "kanagawa"}

	got, _, err := resolveTheme(options{theme: "dayfox"}, cfg)
	if err != nil {
		t.Fatalf("resolveTheme: %v", err)
	}
	if got.Name != "Dayfox" {
		t.Errorf("Name = %q, want Dayfox", got.Name)
	}
}

func TestResolveTheme_UsesConfigWhenNoFlag(t *testing.T) {
	cfg := &config.Config{ThemeName: "kanagawa"}

	got, _, err := resolveTheme(options{}, cfg)
	if err != nil {
		t.Fatalf("resolveTheme: %v", err)
	}
	if got.Name != "Kanagawa" {
		t.Errorf("Name = %q, want Kanagawa", got.Name)
	}
}

func TestResolveTheme_UnknownFlagThemeIsFatal(t *testing.T) {
	if _, _, err := resolveTheme(options{theme: "nope"}, &config.Config{}); err == nil {
		t.Error("an unknown --theme should be an error")
	}
}

func TestResolveTheme_InvalidAppearanceIsFatal(t *testing.T) {
	_, _, err := resolveTheme(options{appearance: "purple"}, &config.Config{})
	if err == nil {
		t.Fatal("an invalid --appearance should be an error")
	}
	if !strings.Contains(err.Error(), "purple") {
		t.Errorf("error does not name the bad value: %v", err)
	}
}

// A bad appearance in the config file must not be fatal, unlike the flag —
// the file may simply have aged out of date.
func TestResolveTheme_InvalidConfigAppearanceIsIgnored(t *testing.T) {
	cfg := &config.Config{Appearance: "purple", ThemeName: "kanagawa"}

	if _, _, err := resolveTheme(options{}, cfg); err != nil {
		t.Errorf("a bad appearance in config should not be fatal: %v", err)
	}
}

func TestResolveTheme_AppearanceSelectsPair(t *testing.T) {
	cfg := &config.Config{ThemeDark: "nightfox", ThemeLight: "dayfox"}

	t.Run("dark", func(t *testing.T) {
		got, _, err := resolveTheme(options{appearance: "dark"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Nightfox" {
			t.Errorf("Name = %q, want Nightfox", got.Name)
		}
	})

	t.Run("light", func(t *testing.T) {
		got, _, err := resolveTheme(options{appearance: "light"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Dayfox" {
			t.Errorf("Name = %q, want Dayfox", got.Name)
		}
	})
}

// detectSystemDark must not query the terminal when there isn't one; the test
// process has no tty, so this also documents the headless default.
func TestDetectSystemDarkWithoutTerminal(t *testing.T) {
	if !detectSystemDark() {
		t.Error("want dark when stdin/stdout are not terminals")
	}
}
