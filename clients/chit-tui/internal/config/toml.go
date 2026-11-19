package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// FileSettings holds the values a config file may supply. Every field is a
// pointer-free zero-value-means-absent string, since an empty setting and a
// missing one behave identically: both defer to the next source.
type FileSettings struct {
	ServerURL   string
	WSScheme    string
	AuthHeader  string
	SessionFile string
	Theme       string
	ThemeDark   string
	ThemeLight  string
	Appearance  string
}

// fileKeys is the set of recognized TOML keys; anything absent from it is
// reported and dropped. setFileKey must handle every one of them.
var fileKeys = []string{
	"server_url",
	"ws_scheme",
	"auth_header",
	"session_file",
	"theme",
	"theme_dark",
	"theme_light",
	"appearance",
}

// isFileKey reports whether key is a recognized setting.
func isFileKey(key string) bool { return slices.Contains(fileKeys, key) }

// setFileKey stores v in the FileSettings field key names. It reports false
// for a key that is not a recognized setting and leaves s unchanged.
func setFileKey(s *FileSettings, key, v string) bool {
	switch key {
	case "server_url":
		s.ServerURL = v
	case "ws_scheme":
		s.WSScheme = v
	case "auth_header":
		s.AuthHeader = v
	case "session_file":
		s.SessionFile = v
	case "theme":
		s.Theme = v
	case "theme_dark":
		s.ThemeDark = v
	case "theme_light":
		s.ThemeLight = v
	case "appearance":
		s.Appearance = v
	default:
		return false
	}
	return true
}

// Dir returns the chit configuration directory, $XDG_CONFIG_HOME/chit.
func Dir() string { return filepath.Join(xdg.ConfigHome, "chit") }

// FilePath returns the config file location. CHIT_CONFIG_FILE overrides it,
// which is what lets tests and one-off invocations point somewhere else.
func FilePath() string {
	if p := os.Getenv("CHIT_CONFIG_FILE"); p != "" {
		return p
	}
	return filepath.Join(Dir(), "config.toml")
}

// ThemesDir returns the directory holding user theme files.
func ThemesDir() string { return filepath.Join(Dir(), "themes") }

// LoadFile reads and validates a config file. It is deliberately forgiving and
// never returns an error: a missing file is normal, and a malformed one falls
// back to defaults with a warning rather than locking the user out of a chat
// client over a typo. Callers surface the warnings; nothing is fatal.
func LoadFile(path string) (FileSettings, []string) {
	var settings FileSettings

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return settings, nil
		}
		return settings, []string{fmt.Sprintf("config %s: %v — using defaults", path, err)}
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return settings, []string{
			fmt.Sprintf("config %s: invalid TOML (%v) — using defaults", path, err),
		}
	}

	var warnings []string
	stripUnknownKeys(raw, &warnings)
	vetConfig(raw, &warnings)
	apply(raw, &settings)

	return settings, warnings
}

// stripUnknownKeys removes keys the schema does not define. They are caught
// here rather than in CUE so the message can name the key plainly; CUE reports
// a closed-struct violation, which reads as noise.
func stripUnknownKeys(raw map[string]any, warnings *[]string) {
	unknown := make([]string, 0)
	for k := range raw {
		if !isFileKey(k) {
			unknown = append(unknown, k)
		}
	}

	// Sorted so the warnings are stable across runs; Go map order is not.
	sort.Strings(unknown)
	for _, k := range unknown {
		delete(raw, k)
		*warnings = append(*warnings, warnKey(k))
	}
}

// apply copies validated values onto settings. Anything that survived vetting
// is known to be a string, so a non-string here is simply skipped.
func apply(raw map[string]any, settings *FileSettings) {
	for _, key := range fileKeys {
		v, ok := raw[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok {
			setFileKey(settings, key, s)
		}
	}
}
