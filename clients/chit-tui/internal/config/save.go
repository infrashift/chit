package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
)

// themeSettingRe matches an existing top-level assignment of the theme
// setting key. Only these keys are ever rewritten: it reports false for any
// other key. It is compiled per call; a theme is saved only when the user
// picks one.
func themeSettingRe(key string) (*regexp.Regexp, bool) {
	switch key {
	case "theme", "theme_dark", "theme_light":
		re := regexp.MustCompile(`(?m)^\s*` + key + `\s*=.*$`)
		// It must find the line SaveThemeSetting writes, or each save would
		// append another assignment of the same key.
		assert.That(re.MatchString(key+` = "x"`), "themeSettingRe: pattern misses the line it writes")
		assert.That(isFileKey(key), "themeSettingRe: key is not a config setting")
		return re, true
	default:
		return nil, false
	}
}

// SaveTheme records a theme choice in the config file so it survives a restart.
func SaveTheme(name string) error { return SaveThemeSetting("theme", name) }

// SaveThemeSetting records a theme choice under key: "theme", or
// "theme_dark" or "theme_light" to choose for one appearance only.
//
// The file is edited line by line rather than decoded and re-encoded: a
// round-trip through the TOML encoder would silently discard the user's
// comments and key ordering, which is a poor trade for persisting one setting.
func SaveThemeSetting(key, name string) error {
	themeLineRe, ok := themeSettingRe(key)
	if !ok {
		return fmt.Errorf("%q is not a theme setting", key)
	}
	assert.That(themeLineRe != nil, "SaveThemeSetting: no pattern for a theme setting")
	path := FilePath()
	assert.That(path != "", "SaveThemeSetting: empty config path")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	line := fmt.Sprintf("%s = %q", key, name)
	assert.That(themeLineRe.MatchString(line), "SaveThemeSetting: pattern misses the line it writes")

	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read config: %w", err)
		}
		// No config yet: write one containing just this setting.
		return writeConfig(path, line+"\n")
	}

	body := string(existing)
	switch {
	case themeLineRe.MatchString(body):
		// Literal: a "$" in the theme name is text, not a group reference.
		body = themeLineRe.ReplaceAllLiteralString(body, line)
	case strings.HasSuffix(body, "\n") || body == "":
		body += line + "\n"
	default:
		body += "\n" + line + "\n"
	}

	assert.That(strings.Contains(body, line), "SaveThemeSetting: the new setting is missing from the file")
	// The next save must find this assignment and replace it.
	assert.That(themeLineRe.MatchString(body), "SaveThemeSetting: the next save would not find the setting")
	return writeConfig(path, body)
}

// writeConfig replaces the config file atomically, so an interrupted write
// cannot leave a truncated file that fails to parse on next start.
func writeConfig(path, body string) error {
	assert.That(path != "", "writeConfig: empty path")
	assert.That(body != "", "writeConfig: empty body")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	assert.That(filepath.Dir(tmpName) == filepath.Dir(path), "writeConfig: temp file must be beside the target for the rename to be atomic")

	// Cleanup is best-effort: the write has already failed by the time it
	// runs, and a leftover temp file is not worth masking the real error.
	discard := func() { errlog.Debug(os.Remove(tmpName), "remove temp config") }

	if _, err := tmp.WriteString(body); err != nil {
		errlog.Debug(tmp.Close(), "close temp config")
		discard()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		discard()
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		discard()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		discard()
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
