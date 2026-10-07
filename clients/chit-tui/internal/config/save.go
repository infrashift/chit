package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// themeSettingRe matches an existing top-level assignment of each theme
// setting. Only these keys are ever rewritten.
var themeSettingRe = map[string]*regexp.Regexp{
	"theme":       regexp.MustCompile(`(?m)^\s*theme\s*=.*$`),
	"theme_dark":  regexp.MustCompile(`(?m)^\s*theme_dark\s*=.*$`),
	"theme_light": regexp.MustCompile(`(?m)^\s*theme_light\s*=.*$`),
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
	themeLineRe, ok := themeSettingRe[key]
	if !ok {
		return fmt.Errorf("%q is not a theme setting", key)
	}
	path := FilePath()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	line := fmt.Sprintf("%s = %q", key, name)

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
		body = themeLineRe.ReplaceAllString(body, line)
	case strings.HasSuffix(body, "\n") || body == "":
		body += line + "\n"
	default:
		body += "\n" + line + "\n"
	}

	return writeConfig(path, body)
}

// writeConfig replaces the config file atomically, so an interrupted write
// cannot leave a truncated file that fails to parse on next start.
func writeConfig(path, body string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()

	// Cleanup is best-effort: the write has already failed by the time it
	// runs, and a leftover temp file is not worth masking the real error.
	discard := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
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
