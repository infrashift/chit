package config

import (
	"errors"
	"os"
)

// Config holds the application configuration.
type Config struct {
	ServerURL    string
	SessionToken string
	WSScheme     string
	ThemeName    string
	AuthHeader   string
	SessionFile  string
}

// defaults
const (
	DefaultWSScheme   = "ws"
	DefaultAuthHeader = "X-Session-Token"
)

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		ServerURL:    os.Getenv("CHIT_SERVER_URL"),
		SessionToken: os.Getenv("CHIT_SESSION_TOKEN"),
		WSScheme:     os.Getenv("CHIT_WS_SCHEME"),
		ThemeName:    os.Getenv("CHIT_THEME"),
		AuthHeader:   os.Getenv("CHIT_AUTH_HEADER"),
		SessionFile:  os.Getenv("CHIT_SESSION_FILE"),
	}

	if cfg.WSScheme == "" {
		cfg.WSScheme = DefaultWSScheme
	}
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = DefaultAuthHeader
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	if c.ServerURL == "" {
		return errors.New("CHIT_SERVER_URL is required")
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
