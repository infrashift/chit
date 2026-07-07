package mcp

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration for the chit-mcp server, loaded from
// CHIT_MCP_-prefixed environment variables. chit-mcp is an HTTP/WebSocket
// client of chitd, authenticating with the agent's Kratos ID via the trusted
// proxy header — the same model as chit-claude.
type Config struct {
	// ServerURL is chitd's base URL (direct, not through Oathkeeper).
	ServerURL string `koanf:"server_url"`
	// AgentKratosID is the Kratos identity ID of the agent user to act as.
	AgentKratosID string `koanf:"agent_kratos_id"`
	// ProxySecret must match chitd's CHIT_TRUSTED_PROXY_SECRET when set.
	ProxySecret string `koanf:"proxy_secret"`
}

// Defaults returns a Config populated with default values.
func Defaults() *Config {
	return &Config{
		ServerURL: "http://localhost:8065",
	}
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

	if cfg.AgentKratosID == "" {
		return nil, fmt.Errorf("CHIT_MCP_AGENT_KRATOS_ID is required")
	}

	return cfg, nil
}
