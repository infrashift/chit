package mcp

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/infrashift/chit/internal/chitclient"
)

// Config holds all configuration for the chit-mcp server, loaded from
// CHIT_MCP_-prefixed environment variables. chit-mcp is an HTTP/WebSocket
// client of chitd and authenticates the same way chit-claude does: either
// with Hydra-issued OAuth2 access tokens through Oathkeeper (preferred), or
// with the agent's Kratos ID via the trusted proxy header.
type Config struct {
	// ServerURL points at Oathkeeper in OAuth2 mode, or directly at chitd in
	// trusted-proxy mode.
	ServerURL string `koanf:"server_url"`
	// AgentKratosID is the Kratos identity ID of the agent user to act as.
	AgentKratosID string `koanf:"agent_kratos_id"`
	// ProxySecret must match chitd's CHIT_TRUSTED_PROXY_SECRET when set.
	ProxySecret string `koanf:"proxy_secret"`

	// OAuth2 client credentials. Setting OAuthClientID switches to token
	// authentication; the agent's identity then comes from the client binding
	// in chitd rather than from AgentKratosID.
	OAuthTokenURL     string `koanf:"oauth_token_url"`
	OAuthClientID     string `koanf:"oauth_client_id"`
	OAuthClientSecret string `koanf:"oauth_client_secret"`
	OAuthScopes       string `koanf:"oauth_scopes"`
	OAuthAudience     string `koanf:"oauth_audience"`
}

// Defaults returns a Config populated with default values.
func Defaults() *Config {
	return &Config{
		ServerURL:     "http://localhost:8065",
		OAuthScopes:   "chit:read chit:write",
		OAuthAudience: "chit",
	}
}

// UseOAuth reports whether chit-mcp authenticates with Hydra-issued access
// tokens rather than the trusted proxy header.
func (c *Config) UseOAuth() bool { return c.OAuthClientID != "" }

// OAuth returns the client-credentials configuration for chitclient.
func (c *Config) OAuth() *chitclient.OAuthConfig {
	return &chitclient.OAuthConfig{
		TokenURL:     c.OAuthTokenURL,
		ClientID:     c.OAuthClientID,
		ClientSecret: c.OAuthClientSecret,
		Scopes:       strings.Fields(c.OAuthScopes),
		Audience:     c.OAuthAudience,
	}
}

// NewClient builds the chitd client for this configuration.
func (c *Config) NewClient() *chitclient.Client {
	if c.UseOAuth() {
		return chitclient.NewOAuth(c.ServerURL, c.OAuth())
	}
	return chitclient.New(c.ServerURL, c.AgentKratosID, c.ProxySecret)
}

// LoadConfig reads configuration from CHIT_MCP_-prefixed environment variables.
func LoadConfig() (*Config, error) {
	k := koanf.New(".")
	cfg := Defaults()

	err := k.Load(env.ProviderWithValue("CHIT_MCP_", ".", func(key, value string) (string, any) {
		return strings.ToLower(strings.TrimPrefix(key, "CHIT_MCP_")), value
	}), nil)
	if err != nil {
		return nil, err
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, err
	}

	// Exactly one authentication mode must be configured; falling back to the
	// weaker one by accident would be worse than refusing to start.
	switch {
	case cfg.UseOAuth():
		if cfg.OAuthClientSecret == "" {
			return nil, fmt.Errorf("CHIT_MCP_OAUTH_CLIENT_SECRET is required when OAUTH_CLIENT_ID is set")
		}
		if cfg.OAuthTokenURL == "" {
			return nil, fmt.Errorf("CHIT_MCP_OAUTH_TOKEN_URL is required when OAUTH_CLIENT_ID is set")
		}
	case cfg.AgentKratosID == "":
		return nil, fmt.Errorf("either CHIT_MCP_OAUTH_CLIENT_ID or CHIT_MCP_AGENT_KRATOS_ID is required")
	}

	return cfg, nil
}
