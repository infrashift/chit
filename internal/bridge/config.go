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
)

// Config holds all configuration for the chit-claude bridge, loaded from
// CHIT_CLAUDE_-prefixed environment variables.
type Config struct {
	// Chit connection
	ServerURL     string `koanf:"server_url"`
	AgentKratosID string `koanf:"agent_kratos_id"`
	ProxySecret   string `koanf:"proxy_secret"`

	// What to listen to
	Channels       []string `koanf:"channels"`
	RequireMention bool     `koanf:"require_mention"`

	// How to run claude
	ClaudeBin          string        `koanf:"claude_bin"`
	WorkDir            string        `koanf:"workdir"`
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

	if cfg.AgentKratosID == "" {
		return nil, fmt.Errorf("CHIT_CLAUDE_AGENT_KRATOS_ID is required")
	}
	if len(cfg.Channels) == 0 {
		return nil, fmt.Errorf("CHIT_CLAUDE_CHANNELS is required (comma-separated channel IDs)")
	}
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("CHIT_CLAUDE_WORKDIR is required")
	}

	return cfg, nil
}
