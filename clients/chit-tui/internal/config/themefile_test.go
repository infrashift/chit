package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

// completeTheme is a minimal valid theme file: every required palette key,
// nothing else.
const completeTheme = `
name = "Test Theme"
author = "tester"
background = "#1a1b26"
foreground = "#c0caf5"
subtle = "#565f89"
accent = "#7aa2f7"
error = "#f7768e"
success = "#9ece6a"
warning = "#e0af68"
border = "#3b4261"
active_border = "#7aa2f7"
highlight = "#292e42"
muted = "#545c7e"
username = "#bb9af7"
timestamp = "#565f89"
unread_badge = "#7aa2f7"
pin_badge = "#e0af68"
channel_active = "#7aa2f7"
mention_badge = "#f7768e"
mention_text = "#7aa2f7"
mention_self_bg = "#e0af68"
tag_badge = "#9ece6a"
`

func TestValidateThemeTOML_Complete(t *testing.T) {
	path := writeFile(t, "ok.toml", completeTheme)

	values, warnings, err := config.ValidateThemeTOML(path)
	if err != nil {
		t.Fatalf("valid theme rejected: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("valid theme warned: %v", warnings)
	}
	for _, key := range config.ThemeColorKeys {
		if values[key] == "" {
			t.Errorf("key %s missing from the returned values", key)
		}
	}
	if values["name"] != "Test Theme" {
		t.Errorf("name = %q", values["name"])
	}
}

func TestValidateThemeTOML_MissingFile(t *testing.T) {
	_, _, err := config.ValidateThemeTOML(filepath.Join(t.TempDir(), "absent.toml"))
	if !errors.Is(err, config.ErrThemeNotFound) {
		t.Errorf("err = %v, want ErrThemeNotFound so callers can tell absent from broken", err)
	}
}

func TestValidateThemeTOML_InvalidTOML(t *testing.T) {
	path := writeFile(t, "bad.toml", "not [ valid")
	if _, _, err := config.ValidateThemeTOML(path); err == nil {
		t.Error("expected an error for invalid TOML")
	}
}

// Every missing key must be reported at once. CUE suppresses required-field
// diagnostics when value errors are also present, so these are detected in Go
// first — otherwise a user fixes one key per run.
func TestValidateThemeTOML_ReportsAllMissingKeysAtOnce(t *testing.T) {
	path := writeFile(t, "sparse.toml", `
name = "Sparse"
background = "#000000"
`)

	_, _, err := config.ValidateThemeTOML(path)
	if err == nil {
		t.Fatal("expected an error for an incomplete theme")
	}

	msg := err.Error()
	for _, key := range config.ThemeColorKeys {
		if key == "background" {
			continue
		}
		if !strings.Contains(msg, key) {
			t.Errorf("error does not mention the missing key %q:\n%s", key, msg)
		}
	}
}

// The regression this guards: a file with one bad value AND missing keys must
// report both, not just the bad value.
func TestValidateThemeTOML_ReportsMissingKeysAlongsideBadValues(t *testing.T) {
	path := writeFile(t, "mixed.toml", `
name = "Mixed"
background = "not-a-color"
`)

	_, _, err := config.ValidateThemeTOML(path)
	if err == nil {
		t.Fatal("expected an error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "background") {
		t.Errorf("error does not mention the bad value:\n%s", msg)
	}
	if !strings.Contains(msg, "foreground") {
		t.Errorf("error does not mention a missing key:\n%s", msg)
	}
}

func TestValidateThemeTOML_BadColorMessageIsHumanReadable(t *testing.T) {
	path := writeFile(t, "badcolor.toml", strings.Replace(
		completeTheme, `accent = "#7aa2f7"`, `accent = "bright-blue"`, 1))

	_, _, err := config.ValidateThemeTOML(path)
	if err == nil {
		t.Fatal("expected an error for a non-hex color")
	}

	msg := err.Error()
	if !strings.Contains(msg, "accent") {
		t.Errorf("error does not name the key:\n%s", msg)
	}
	if !strings.Contains(msg, "#7aa2f7") {
		t.Errorf("error does not show the expected format:\n%s", msg)
	}
	for _, leak := range []string{"#Theme", "conflicting values", "cue:"} {
		if strings.Contains(msg, leak) {
			t.Errorf("error leaks CUE internals (%q):\n%s", leak, msg)
		}
	}
}

func TestValidateThemeTOML_RejectsShorthandAndBadHex(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "six digit", value: "#7aa2f7", valid: true},
		{name: "three digit", value: "#abc", valid: true},
		{name: "uppercase", value: "#7AA2F7", valid: true},
		{name: "no hash", value: "7aa2f7", valid: false},
		{name: "five digit", value: "#7aa2f", valid: false},
		{name: "non hex", value: "#zzzzzz", valid: false},
		{name: "named color", value: "blue", valid: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(completeTheme,
				`accent = "#7aa2f7"`, fmt.Sprintf("accent = %q", tc.value), 1)
			path := writeFile(t, "c.toml", body)

			_, _, err := config.ValidateThemeTOML(path)
			if tc.valid && err != nil {
				t.Errorf("%q rejected: %v", tc.value, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("%q accepted", tc.value)
			}
		})
	}
}

// An unrecognized key is most likely left over from an older version, so it
// warns rather than refusing to load the theme.
func TestValidateThemeTOML_UnknownKeyWarnsButLoads(t *testing.T) {
	path := writeFile(t, "extra.toml", completeTheme+"\nsyntax_style = \"monokai\"\n")

	values, warnings, err := config.ValidateThemeTOML(path)
	if err != nil {
		t.Fatalf("an unknown key should not be fatal: %v", err)
	}
	if !containsSubstring(warnings, "syntax_style") {
		t.Errorf("warnings do not mention the unknown key: %v", warnings)
	}
	if _, ok := values["syntax_style"]; ok {
		t.Error("unknown key was returned to the caller")
	}
}

func TestValidateThemeTOML_DerivedKeysAreOptional(t *testing.T) {
	t.Run("omitted", func(t *testing.T) {
		path := writeFile(t, "d.toml", completeTheme)
		values, _, err := config.ValidateThemeTOML(path)
		if err != nil {
			t.Fatalf("derived keys should be optional: %v", err)
		}
		if _, ok := values["selection"]; ok {
			t.Error("selection should be absent when not declared")
		}
	})

	t.Run("supplied", func(t *testing.T) {
		path := writeFile(t, "d.toml", completeTheme+"\nselection = \"#123456\"\n")
		values, _, err := config.ValidateThemeTOML(path)
		if err != nil {
			t.Fatalf("explicit derived key rejected: %v", err)
		}
		if values["selection"] != "#123456" {
			t.Errorf("selection = %q", values["selection"])
		}
	})
}

func TestValidateThemeTOML_NameIsRequired(t *testing.T) {
	body := strings.Replace(completeTheme, "name = \"Test Theme\"\n", "", 1)
	path := writeFile(t, "noname.toml", body)

	_, _, err := config.ValidateThemeTOML(path)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("a theme without a name should be rejected, got: %v", err)
	}
}

// The palette key list is duplicated between ThemeColorKeys and the CUE
// schema, and nothing but this test keeps them in step. A slot added to one
// and not the other silently stops being validated.
func TestThemeColorKeysMatchSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("schema", "theme.cue"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	src := string(data)

	for _, key := range config.ThemeColorKeys {
		if !strings.Contains(src, key+"!:") {
			t.Errorf("key %q is in ThemeColorKeys but not required by schema/theme.cue", key)
		}
	}
	for _, key := range config.ThemeDerivedKeys {
		if !strings.Contains(src, key+"?:") {
			t.Errorf("key %q is in ThemeDerivedKeys but not optional in schema/theme.cue", key)
		}
	}

	// name is required too, so the schema should hold exactly the palette keys
	// plus name as required fields.
	if got, want := strings.Count(src, "!:"), len(config.ThemeColorKeys)+1; got != want {
		t.Errorf("schema has %d required keys, want %d (palette + name); "+
			"a key was added to the schema without updating ThemeColorKeys", got, want)
	}
}
