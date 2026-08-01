package config

import (
	"fmt"
	"os"
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

	cfg := &Config{
		ServerURL:    firstNonEmpty(os.Getenv("CHIT_SERVER_URL"), file.ServerURL),
		SessionToken: os.Getenv("CHIT_SESSION_TOKEN"),
		WSScheme:     firstNonEmpty(os.Getenv("CHIT_WS_SCHEME"), file.WSScheme, DefaultWSScheme),
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
