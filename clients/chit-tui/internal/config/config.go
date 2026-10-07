package config

import (
	"fmt"
	"os"
	"strings"
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
	}

	if err := cfg.Validate(); err != nil {
		return nil, warnings, err
	}

	return cfg, warnings, nil
}

// defaultWSScheme matches the WebSocket to the server's own security. A
// fixed "ws" failed to dial an HTTPS server, or sent the session token in the
// clear.
func defaultWSScheme(serverURL string) string {
	if strings.HasPrefix(strings.ToLower(serverURL), "https://") {
		return "wss"
	}
	return DefaultWSScheme
}

// firstNonEmpty returns the first value that is set, which is how each setting
// walks its precedence chain.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
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
	if c.ThemeName != "" || (c.ThemeDark == "" && c.ThemeLight == "") {
		return "theme"
	}
	if dark {
		return "theme_dark"
	}
	return "theme_light"
}

// HasToken returns true if a session token is configured.
func (c *Config) HasToken() bool {
	return c.SessionToken != ""
}

// KratosBaseURL returns the Kratos API base URL derived from ServerURL.
func (c *Config) KratosBaseURL() string {
	return c.ServerURL + "/kratos"
}

// WSURL returns the WebSocket URL derived from the server URL.
func (c *Config) WSURL() string {
	return c.WSScheme + "://" + stripScheme(c.ServerURL) + "/api/v1/websocket"
}

func stripScheme(url string) string {
	for _, prefix := range []string{"https://", "http://", "ws://", "wss://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
