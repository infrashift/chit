package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	// ThemeWatch and ThemePaint hold "true" or "false" from a TOML boolean,
	// or "" when the file does not set them.
	ThemeWatch string
	ThemePaint string
	// DownloadDir is where the client saves the files of a post.
	DownloadDir string
}

// fileKeys returns the recognized TOML keys; anything absent from them is
// reported and dropped. setFileKey must handle every one of them.
func fileKeys() []string {
	return []string{
		"server_url",
		"ws_scheme",
		"auth_header",
		"session_file",
		"theme",
		"theme_dark",
		"theme_light",
		"appearance",
		"theme_watch",
		"theme_paint",
		"download_dir",
	}
}

// isFileKey reports whether key is a recognized setting.
func isFileKey(key string) bool { return slices.Contains(fileKeys(), key) }

// setFileKey stores v in the FileSettings field key names. It reports false
// for a key that is not a recognized setting and leaves s unchanged.
func setFileKey(s *FileSettings, key, v string) bool {
	assert.That(s != nil, "setFileKey: nil settings")
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
	case "theme_watch":
		s.ThemeWatch = v
	case "theme_paint":
		s.ThemePaint = v
	case "download_dir":
		s.DownloadDir = v
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
	p := filepath.Join(Dir(), "config.toml")
	assert.That(filepath.Dir(p) == filepath.Clean(Dir()), "FilePath: default file must be in the config directory")
	return p
}

// ThemesDir returns the directory holding user theme files.
func ThemesDir() string { return filepath.Join(Dir(), "themes") }

// LoadFile reads and validates a config file. It is deliberately forgiving and
// never returns an error: a missing file is normal, and a malformed one falls
// back to defaults with a warning rather than locking the user out of a chat
// client over a typo. Callers surface the warnings; nothing is fatal.
func LoadFile(path string) (FileSettings, []string) {
	assert.That(path != "", "LoadFile: empty path")
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

	assert.That(raw != nil, "LoadFile: a decoded document must be a table")
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
	assert.That(raw != nil, "stripUnknownKeys: nil document")
	assert.That(warnings != nil, "stripUnknownKeys: nil warnings")
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
	assert.That(len(*warnings) >= len(unknown), "stripUnknownKeys: a dropped key went unreported")
	assert.That(len(raw) <= len(fileKeys()), "stripUnknownKeys: unknown keys remain")
}

// apply copies validated values onto settings. Anything that survived vetting
// is a string or, for the switches, a boolean. A boolean is stored as
// "true" or "false".
func apply(raw map[string]any, settings *FileSettings) {
	assert.That(raw != nil, "apply: nil document")
	assert.That(settings != nil, "apply: nil settings")
	for _, key := range fileKeys() {
		v, ok := raw[key]
		if !ok {
			continue
		}
		if b, ok := v.(bool); ok {
			v = strconv.FormatBool(b)
		}
		if s, ok := v.(string); ok {
			handled := setFileKey(settings, key, s)
			assert.That(handled, "apply: setFileKey does not handle a recognized key")
		}
	}
}

// DefaultDownloadDir is the XDG download directory, or ~/Downloads when XDG
// names none.
func DefaultDownloadDir() string {
	if d := xdg.UserDirs.Download; d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "Downloads"
	}
	out := filepath.Join(home, "Downloads")
	assert.That(filepath.IsAbs(out), "DefaultDownloadDir: not an absolute path")
	return out
}
