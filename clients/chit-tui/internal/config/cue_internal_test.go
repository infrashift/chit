package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These cover the defensive paths in the validation pipeline, which are
// unreachable from outside the package: they only fire when the embedded
// schema or the decoded document is broken in a way a TOML file cannot
// express.

func TestClearMap(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2}
	clearMap(m)
	if len(m) != 0 {
		t.Errorf("map still holds %d keys", len(m))
	}
}

func TestTrimDefinitionPrefix(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want string
	}{
		{name: "definition prefix stripped", path: []string{"#Config", "theme"}, want: "theme"},
		{name: "bare key", path: []string{"appearance"}, want: "appearance"},
		{name: "nested under definition", path: []string{"#Theme", "accent", "0"}, want: "accent"},
		{name: "empty", path: nil, want: ""},
		{name: "only definitions", path: []string{"#Config"}, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := trimDefinitionPrefix(tc.path); got != tc.want {
				t.Errorf("trimDefinitionPrefix(%v) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestWarnKey(t *testing.T) {
	t.Run("known key explains the constraint", func(t *testing.T) {
		got := warnKey("appearance")
		if !strings.Contains(got, `"dark", "light", or "system"`) {
			t.Errorf("warnKey(appearance) = %q", got)
		}
	})

	t.Run("unknown key says so plainly", func(t *testing.T) {
		got := warnKey("nonsense")
		if !strings.Contains(got, "not a recognized setting") {
			t.Errorf("warnKey(nonsense) = %q", got)
		}
	})
}

// Every key the schema accepts needs a human message, or a user hitting it
// gets the bare fallback text instead of an explanation.
func TestEveryFileKeyHasAMessage(t *testing.T) {
	for key := range fileKeys {
		if _, ok := keyMessages[key]; !ok {
			t.Errorf("key %q is accepted by the schema but has no entry in keyMessages", key)
		}
	}
}

func TestCompileSchemaRejectsBadSource(t *testing.T) {
	if _, err := compileSchema("this is not ( cue", "#Config"); err == nil {
		t.Error("expected an error for unparseable CUE")
	}
}

func TestCompileSchemaRejectsMissingDefinition(t *testing.T) {
	if _, err := compileSchema(configSchemaSrc, "#NotADefinition"); err == nil {
		t.Error("expected an error for a definition that is not in the schema")
	}
}

// Both embedded schemas must compile; a typo in either would otherwise only
// surface at runtime as a skipped-validation warning.
func TestEmbeddedSchemasCompile(t *testing.T) {
	if _, err := compileSchema(configSchemaSrc, "#Config"); err != nil {
		t.Errorf("config schema does not compile: %v", err)
	}
	if _, err := compileSchema(themeSchemaSrc, "#Theme"); err != nil {
		t.Errorf("theme schema does not compile: %v", err)
	}
}

// dropFirstOffender gives up when it cannot trace an error back to a key that
// is actually present, which is what tips vetConfig into discarding the
// document rather than looping.
func TestDropFirstOffenderGivesUpOnUntraceableError(t *testing.T) {
	raw := map[string]any{"theme": "kanagawa"}
	var warnings []string

	if dropFirstOffender(raw, errUntraceable{}, &warnings) {
		t.Error("expected false for an error with no usable path")
	}
	if len(raw) != 1 {
		t.Errorf("raw was modified: %v", raw)
	}
}

type errUntraceable struct{}

func (errUntraceable) Error() string { return "no path here" }

func TestFilePathDefaultsUnderConfigDir(t *testing.T) {
	t.Setenv("CHIT_CONFIG_FILE", "")

	got := FilePath()
	if filepath.Base(got) != "config.toml" {
		t.Errorf("FilePath() = %q, want it to end in config.toml", got)
	}
	if !strings.HasPrefix(got, Dir()) {
		t.Errorf("FilePath() = %q, want it under %q", got, Dir())
	}
}

// A directory where a config file is expected must not crash the client.
func TestLoadFileOnDirectory(t *testing.T) {
	dir := t.TempDir()

	settings, warnings := LoadFile(dir)
	if len(warnings) == 0 {
		t.Error("expected a warning when the path is a directory")
	}
	if settings.Theme != "" {
		t.Errorf("Theme = %q, want empty", settings.Theme)
	}
}

func TestValidateThemeTOMLOnDirectory(t *testing.T) {
	if _, _, err := ValidateThemeTOML(t.TempDir()); err == nil {
		t.Error("expected an error when the theme path is a directory")
	}
}

func TestStripUnknownThemeKeysSortsWarnings(t *testing.T) {
	raw := map[string]any{"zebra": "1", "alpha": "2", "name": "keep"}

	warnings := stripUnknownThemeKeys(raw)
	if len(warnings) != 2 {
		t.Fatalf("got %d warnings, want 2: %v", len(warnings), warnings)
	}
	// Sorted so repeated runs produce identical output; Go map order is not.
	if !strings.Contains(warnings[0], "alpha") || !strings.Contains(warnings[1], "zebra") {
		t.Errorf("warnings are not sorted: %v", warnings)
	}
	if _, ok := raw["name"]; !ok {
		t.Error("a known key was stripped")
	}
}

func TestKnownThemeKeysCoversEveryList(t *testing.T) {
	known := knownThemeKeys()
	for _, group := range [][]string{ThemeColorKeys, ThemeDerivedKeys, ThemeMeta} {
		for _, key := range group {
			if !known[key] {
				t.Errorf("key %q is not in knownThemeKeys", key)
			}
		}
	}
}

// The embedded schema must match the file on disk, or the compiled binary
// validates against something the repository no longer describes.
func TestEmbeddedSchemaMatchesFile(t *testing.T) {
	onDisk, err := os.ReadFile(filepath.Join("schema", "config.cue"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if string(onDisk) != configSchemaSrc {
		t.Error("embedded config schema differs from schema/config.cue")
	}
}
