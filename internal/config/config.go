package config

import (
	"log"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration for the chit server.
type Config struct {
	ListenAddress   string `koanf:"listen_address"`
	DatabaseURL     string `koanf:"database_url"`
	DBMaxOpenConn   int    `koanf:"db_max_open_conn"`
	DBMaxIdleConn   int    `koanf:"db_max_idle_conn"`

	PubSubBackend string `koanf:"pubsub_backend"`
	NatsURL       string `koanf:"nats_url"`

	TrustedProxyHeader string `koanf:"trusted_proxy_header"`
	KratosAdminURL     string `koanf:"kratos_admin_url"`
	KetoReadURL        string `koanf:"keto_read_url"`
	KetoWriteURL       string `koanf:"keto_write_url"`

	ZincSearchURL      string `koanf:"zincsearch_url"`
	ZincSearchUser     string `koanf:"zincsearch_user"`
	ZincSearchPassword string `koanf:"zincsearch_password"`

	WSPingInterval time.Duration `koanf:"ws_ping_interval"`
	WSWriteTimeout time.Duration `koanf:"ws_write_timeout"`

	CacheSize int           `koanf:"cache_size"`
	CacheTTL  time.Duration `koanf:"cache_ttl"`

	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`

	EnableOpenAPIValidation bool   `koanf:"enable_openapi_validation"`
	MCPAgentUserID          string `koanf:"mcp_agent_user_id"`

	// Slash Commands
	CommandsCUEDir     string `koanf:"commands_cue_dir"`
	AuditLogPath       string `koanf:"audit_log_path"`
	WebhookEnabled     bool   `koanf:"webhook_enabled"`
	WebhookURL         string `koanf:"webhook_url"`
	WebhookSecret      string `koanf:"webhook_secret"`
	WebhookWorkerCount int    `koanf:"webhook_worker_count"`
	WebhookQueueSize   int    `koanf:"webhook_queue_size"`
	WebhookTimeoutSec  int    `koanf:"webhook_timeout_sec"`
}

// Defaults returns a Config populated with default values.
func Defaults() *Config {
	return &Config{
		ListenAddress:      ":8065",
		DBMaxOpenConn:      25,
		DBMaxIdleConn:      10,
		PubSubBackend:      "pgnotify",
		NatsURL:            "nats://localhost:4222",
		TrustedProxyHeader: "X-User-Id",
		KratosAdminURL:     "http://localhost:4434",
		KetoReadURL:        "http://localhost:4466",
		KetoWriteURL:       "http://localhost:4467",
		ZincSearchURL:      "http://localhost:4080",
		ZincSearchUser:     "admin",
		WSPingInterval:     30 * time.Second,
		WSWriteTimeout:     10 * time.Second,
		CacheSize:          20000,
		CacheTTL:           5 * time.Minute,
		LogLevel:                "info",
		LogFormat:               "json",
		EnableOpenAPIValidation: true,
		CommandsCUEDir:         "auth",
		AuditLogPath:           "/var/log/chit/audit.log",
		WebhookWorkerCount:     4,
		WebhookQueueSize:       1024,
		WebhookTimeoutSec:      10,
	}
}

// Load reads configuration from environment variables prefixed with CHIT_.
func Load() (*Config, error) {
	k := koanf.New(".")
	cfg := Defaults()

	err := k.Load(env.Provider("CHIT_", ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, "CHIT_"))
	}), nil)
	if err != nil {
		return nil, err
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, err
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("CHIT_DATABASE_URL is required")
	}

	return cfg, nil
}
