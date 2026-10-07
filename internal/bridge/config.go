// Package bridge connects Chit channels to headless Claude Code sessions.
// It listens for posts over chitd's WebSocket, runs `claude -p` (resuming the
// thread's session when one exists), and posts the result back into the
// thread as the agent user — making Chit an asynchronous, markdown-native
// front-end for Claude Code.
package bridge

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/infrashift/chit/internal/chitclient"
)

// Config holds all configuration for the chit-claude bridge, loaded from
// CHIT_CLAUDE_-prefixed environment variables.
type Config struct {
	// Chit connection. In OAuth2 mode ServerURL points at Oathkeeper (which
	// introspects the token); in trusted-proxy mode it points at chitd.
	ServerURL     string `koanf:"server_url"`
	AgentKratosID string `koanf:"agent_kratos_id"`
	ProxySecret   string `koanf:"proxy_secret"`

	// OAuth2 client credentials. Setting OAuthClientID switches the bridge to
	// token authentication; the agent's identity then comes from the client
	// binding in chitd rather than from AgentKratosID.
	OAuthTokenURL     string `koanf:"oauth_token_url"`
	OAuthClientID     string `koanf:"oauth_client_id"`
	OAuthClientSecret string `koanf:"oauth_client_secret"`
	OAuthScopes       string `koanf:"oauth_scopes"`
	OAuthAudience     string `koanf:"oauth_audience"`

	// What to listen to
	Channels       []string `koanf:"channels"`
	RequireMention bool     `koanf:"require_mention"`

	// Multi-agent behaviour. Several bridges (one per persona) can serve the
	// same channel; these knobs keep them from answering each other forever.
	// ReplyToAgents off means posts by agent/bot actors are dropped outright.
	// When on, an agent-authored post is answered only if it mentions this
	// agent by name and the thread is still under MaxAgentHops.
	ReplyToAgents bool `koanf:"reply_to_agents"`
	MaxAgentHops  int  `koanf:"max_agent_hops"`

	// How to run claude
	ClaudeBin          string        `koanf:"claude_bin"`
	WorkDir            string        `koanf:"workdir"`
	Model              string        `koanf:"model"`
	PermissionMode     string        `koanf:"permission_mode"`
	AllowedTools       string        `koanf:"allowed_tools"`
	AppendSystemPrompt string        `koanf:"append_system_prompt"`
	RunTimeout         time.Duration `koanf:"run_timeout"`

	// ContextWindow is the assumed model context size (tokens) used for the
	// "context ~N%" footer estimate.
	ContextWindow int `koanf:"context_window"`
}

// Defaults returns a Config populated with default values.
func Defaults() *Config {
	return &Config{
		ServerURL:      "http://localhost:8065",
		ClaudeBin:      "claude",
		PermissionMode: "dontAsk",
		AllowedTools:   "Read,Grep,Glob",
		RunTimeout:     30 * time.Minute,
		ContextWindow:  200_000,
		MaxAgentHops:   2,
		OAuthScopes:    "chit:read chit:write",
		OAuthAudience:  "chit",
	}
}

// UseOAuth reports whether the bridge authenticates with Hydra-issued access
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

// Load reads configuration from CHIT_CLAUDE_-prefixed environment variables.
func Load() (*Config, error) {
	k := koanf.New(".")
	cfg := Defaults()

	err := k.Load(env.ProviderWithValue("CHIT_CLAUDE_", ".", func(key, value string) (string, any) {
		key = strings.ToLower(strings.TrimPrefix(key, "CHIT_CLAUDE_"))
		if key == "channels" {
			parts := strings.Split(value, ",")
			channels := make([]string, 0, len(parts))
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					channels = append(channels, p)
				}
			}
			return key, channels
		}
		return key, value
	}), nil)
	if err != nil {
		return nil, err
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, err
	}

	// Either authentication mode is acceptable, but exactly one must be
	// configured — silently falling back to the weaker one would be worse
	// than refusing to start.
	switch {
	case cfg.UseOAuth():
		if cfg.OAuthClientSecret == "" {
			return nil, fmt.Errorf("CHIT_CLAUDE_OAUTH_CLIENT_SECRET is required when OAUTH_CLIENT_ID is set")
		}
		if cfg.OAuthTokenURL == "" {
			return nil, fmt.Errorf("CHIT_CLAUDE_OAUTH_TOKEN_URL is required when OAUTH_CLIENT_ID is set")
		}
	case cfg.AgentKratosID == "":
		return nil, fmt.Errorf("either CHIT_CLAUDE_OAUTH_CLIENT_ID or CHIT_CLAUDE_AGENT_KRATOS_ID is required")
	}
	if len(cfg.Channels) == 0 {
		return nil, fmt.Errorf("CHIT_CLAUDE_CHANNELS is required (comma-separated channel IDs)")
	}
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("CHIT_CLAUDE_WORKDIR is required")
	}

	return cfg, nil
}
