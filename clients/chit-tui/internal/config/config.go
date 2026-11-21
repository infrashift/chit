package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// Config holds the application configuration.
type Config struct {
	ServerURL    string
	SessionToken string
	WSScheme     string
	ThemeName    string
	ThemeDark    string
	ThemeLight   string
	Appearance   string
	AuthHeader   string
	SessionFile  string
	// ThemeWatch reloads an edited local theme file while the client runs.
	ThemeWatch bool
	// ThemePaint sets the terminal's default colors to the theme's.
	ThemePaint bool
	// DownloadDir is where the client saves the files of a post.
	DownloadDir string
}

// defaults
const (
	// DefaultWSScheme is the WebSocket scheme for a plain-HTTP server; an
	// HTTPS server defaults to "wss".
	DefaultWSScheme   = "ws"
	DefaultAuthHeader = "X-Session-Token"
)

// Load reads configuration from the config file and the environment.
//
// Precedence is environment > config file > built-in default, the usual
// ordering: the file is the persistent choice, an environment variable is a
// deliberate override for one invocation and so wins. Warnings describe
// settings that were ignored; they are advisory and never fatal.
func Load() (*Config, []string, error) {
	file, warnings := LoadFile(FilePath())

	// Paths are appended to the server URL, so a trailing slash doubled up.
	serverURL := strings.TrimRight(firstNonEmpty(os.Getenv("CHIT_SERVER_URL"), file.ServerURL), "/")

	cfg := &Config{
		ServerURL:    serverURL,
		SessionToken: os.Getenv("CHIT_SESSION_TOKEN"),
		WSScheme:     firstNonEmpty(os.Getenv("CHIT_WS_SCHEME"), file.WSScheme, defaultWSScheme(serverURL)),
		ThemeName:    firstNonEmpty(os.Getenv("CHIT_THEME"), file.Theme),
		ThemeDark:    file.ThemeDark,
		ThemeLight:   file.ThemeLight,
		Appearance:   file.Appearance,
		AuthHeader:   firstNonEmpty(os.Getenv("CHIT_AUTH_HEADER"), file.AuthHeader, DefaultAuthHeader),
		SessionFile:  firstNonEmpty(os.Getenv("CHIT_SESSION_FILE"), file.SessionFile),
		ThemeWatch:   file.ThemeWatch != "false",
		ThemePaint:   file.ThemePaint != "false",
		DownloadDir:  firstNonEmpty(os.Getenv("CHIT_DOWNLOAD_DIR"), file.DownloadDir, DefaultDownloadDir()),
	}

	if err := cfg.Validate(); err != nil {
		return nil, warnings, err
	}

	// Each of these ends in a non-empty default, and the server URL has just
	// been validated and trimmed.
	assert.That(cfg.ServerURL != "" && !strings.HasSuffix(cfg.ServerURL, "/"), "Load: server URL must be set and have no trailing slash")
	assert.That(cfg.WSScheme != "", "Load: empty WebSocket scheme")
	assert.That(cfg.AuthHeader != "", "Load: empty auth header")
	return cfg, warnings, nil
}

// defaultWSScheme matches the WebSocket to the server's own security. A
// fixed "ws" failed to dial an HTTPS server, or sent the session token in the
// clear.
func defaultWSScheme(serverURL string) string {
	scheme := DefaultWSScheme
	if strings.HasPrefix(strings.ToLower(serverURL), "https://") {
		scheme = "wss"
	}
	assert.That(scheme == "ws" || scheme == "wss", "defaultWSScheme: scheme must be ws or wss")
	return scheme
}

// firstNonEmpty returns the first value that is set, which is how each setting
// walks its precedence chain.
func firstNonEmpty(values ...string) string {
	// A chain has a source to fall back to.
	assert.That(len(values) >= 2, "firstNonEmpty: a precedence chain needs at least two sources")
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	assert.That(c != nil, "Config.Validate: nil config")
	if c.ServerURL == "" {
		return fmt.Errorf("no server URL configured: set CHIT_SERVER_URL, "+
			"or add server_url to %s", FilePath())
	}
	return nil
}

// ThemeSettingKey names the setting a theme chosen in the client is saved
// under. With themes configured per appearance and no fixed theme, it is the
// one for the current appearance: saving "theme" would outrank both and turn
// appearance switching off for good. Otherwise it is "theme".
func (c *Config) ThemeSettingKey(dark bool) string {
	assert.That(c != nil, "Config.ThemeSettingKey: nil config")
	key := "theme_light"
	switch {
	case c.ThemeName != "" || (c.ThemeDark == "" && c.ThemeLight == ""):
		key = "theme"
	case dark:
		key = "theme_dark"
	}
	// SaveThemeSetting accepts exactly these keys.
	_, ok := themeSettingRe(key)
	assert.That(ok, "Config.ThemeSettingKey: key is not a theme setting")
	return key
}

// HasToken returns true if a session token is configured.
func (c *Config) HasToken() bool {
	assert.That(c != nil, "Config.HasToken: nil config")
	return c.SessionToken != ""
}

// KratosBaseURL returns the Kratos API base URL derived from ServerURL.
func (c *Config) KratosBaseURL() string {
	assert.That(c != nil, "Config.KratosBaseURL: nil config")
	assert.That(c.ServerURL != "", "Config.KratosBaseURL: no server URL")
	return c.ServerURL + "/kratos"
}

// WSURL returns the WebSocket URL derived from the server URL.
func (c *Config) WSURL() string {
	assert.That(c != nil, "Config.WSURL: nil config")
	assert.That(c.ServerURL != "", "Config.WSURL: no server URL")
	assert.That(c.WSScheme != "", "Config.WSURL: no WebSocket scheme")
	return c.WSScheme + "://" + stripScheme(c.ServerURL) + "/api/v1/websocket"
}

func stripScheme(url string) string {
	rest := url
	for _, prefix := range []string{"https://", "http://", "ws://", "wss://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			rest = url[len(prefix):]
			break
		}
	}
	assert.That(strings.HasSuffix(url, rest), "stripScheme: result must be a suffix of the URL")
	assert.That(rest != "" || url == "", "stripScheme: a URL must not be stripped to nothing")
	return rest
}
