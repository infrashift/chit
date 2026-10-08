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

	// What the run inherits from the host. Runs always pass
	// --strict-mcp-config, so MCPConfig (a path or JSON string handed to
	// --mcp-config) is the only way to give Claude MCP servers.
	// SettingSources is passed to --setting-sources; SettingSourcesAll omits
	// the flag and lets the CLI load every source, the host user's included.
	MCPConfig      string `koanf:"mcp_config"`
	SettingSources string `koanf:"setting_sources"`

	// MaxConcurrentRuns caps claude processes across all threads. Each thread
	// already runs one at a time; this bounds how many threads run at once.
	MaxConcurrentRuns int `koanf:"max_concurrent_runs"`

	// ShutdownGrace is how long runs in progress may keep going after a
	// shutdown signal before they are killed. Keep it under the supervisor's
	// own stop timeout (systemd's TimeoutStopSec defaults to 90s).
	ShutdownGrace time.Duration `koanf:"shutdown_grace"`

	// ContextWindow is the assumed model context size (tokens) used for the
	// "context ~N%" footer estimate.
	ContextWindow int `koanf:"context_window"`
}

// SettingSourcesAll is the CHIT_CLAUDE_SETTING_SOURCES value that passes no
// --setting-sources flag at all.
const SettingSourcesAll = "all"

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

		SettingSources:    "project",
		MaxConcurrentRuns: 2,
		ShutdownGrace:     60 * time.Second,
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
		// Both set used to mean OAuth, silently: an operator who thought the
		// agent ran as its Kratos identity would find it acting as another
		// user, with other roles.
		if cfg.AgentKratosID != "" {
			return nil, fmt.Errorf("set CHIT_CLAUDE_OAUTH_CLIENT_ID or CHIT_CLAUDE_AGENT_KRATOS_ID, not both")
		}
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
	if err := cfg.validateLimits(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateLimits rejects numeric settings that would parse but break the
// bridge quietly: a zero timeout fails every run the moment it starts, and a
// zero concurrency cap never starts one.
func (c *Config) validateLimits() error {
	switch {
	case c.RunTimeout <= 0:
		return fmt.Errorf("CHIT_CLAUDE_RUN_TIMEOUT must be positive, got %s", c.RunTimeout)
	case c.MaxConcurrentRuns < 1:
		return fmt.Errorf("CHIT_CLAUDE_MAX_CONCURRENT_RUNS must be at least 1, got %d", c.MaxConcurrentRuns)
	case c.MaxAgentHops < 0:
		return fmt.Errorf("CHIT_CLAUDE_MAX_AGENT_HOPS must not be negative, got %d", c.MaxAgentHops)
	case c.ShutdownGrace < 0:
		return fmt.Errorf("CHIT_CLAUDE_SHUTDOWN_GRACE must not be negative, got %s", c.ShutdownGrace)
	case c.ContextWindow < 0:
		return fmt.Errorf("CHIT_CLAUDE_CONTEXT_WINDOW must not be negative, got %d", c.ContextWindow)
	case strings.TrimSpace(c.SettingSources) == "":
		return fmt.Errorf("CHIT_CLAUDE_SETTING_SOURCES must name sources (e.g. project) or be %q", SettingSourcesAll)
	}
	return nil
}
