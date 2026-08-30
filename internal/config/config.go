package config

import (
	"errors"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration for the chit server.
type Config struct {
	ListenAddress string `koanf:"listen_address"`
	DatabaseURL   string `koanf:"database_url"`
	DBMaxOpenConn int    `koanf:"db_max_open_conn"`
	DBMaxIdleConn int    `koanf:"db_max_idle_conn"`

	PubSubBackend string `koanf:"pubsub_backend"`
	NatsURL       string `koanf:"nats_url"`

	TrustedProxyHeader string `koanf:"trusted_proxy_header"`
	// TrustedClientHeader carries the OAuth2 client_id of a machine actor
	// authenticated by Oathkeeper's introspection of a Hydra
	// client-credentials token. When present it takes precedence over
	// TrustedProxyHeader, and is resolved against users.oauth_client_id with
	// no just-in-time provisioning.
	TrustedClientHeader string `koanf:"trusted_client_header"`
	// TrustedProxySecret, when set, must be presented by the auth proxy in the
	// X-Proxy-Secret header before the trusted proxy header is honored.
	TrustedProxySecret string `koanf:"trusted_proxy_secret"`

	// MachineActors declares the non-human callers, as a JSON array. A machine
	// authenticates with client_credentials and holds no Kratos identity, so
	// nothing can vouch for it the way ProvisionUser does for a person — an
	// undeclared client is authenticated and nobody. See app.MachineActor.
	MachineActors string `koanf:"machine_actors"`
	// AllowedOrigins is the CORS / WebSocket origin allowlist
	// (comma-separated in CHIT_ALLOWED_ORIGINS). "*" allows any origin.
	AllowedOrigins []string `koanf:"allowed_origins"`
	KratosAdminURL string   `koanf:"kratos_admin_url"`
	KetoReadURL    string   `koanf:"keto_read_url"`
	KetoWriteURL   string   `koanf:"keto_write_url"`

	ZincSearchURL      string `koanf:"zincsearch_url"`
	ZincSearchUser     string `koanf:"zincsearch_user"`
	ZincSearchPassword string `koanf:"zincsearch_password"`

	WSPingInterval time.Duration `koanf:"ws_ping_interval"`
	WSWriteTimeout time.Duration `koanf:"ws_write_timeout"`

	CacheSize int           `koanf:"cache_size"`
	CacheTTL  time.Duration `koanf:"cache_ttl"`

	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`

	EnableOpenAPIValidation bool `koanf:"enable_openapi_validation"`

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
		ListenAddress:           ":8065",
		DBMaxOpenConn:           25,
		DBMaxIdleConn:           10,
		PubSubBackend:           "pgnotify",
		NatsURL:                 "nats://localhost:4222",
		TrustedProxyHeader:      "X-User-Id",
		TrustedClientHeader:     "X-Client-Id",
		AllowedOrigins:          []string{"*"},
		KratosAdminURL:          "http://localhost:4434",
		KetoReadURL:             "http://localhost:4466",
		KetoWriteURL:            "http://localhost:4467",
		ZincSearchURL:           "http://localhost:4080",
		ZincSearchUser:          "admin",
		WSPingInterval:          30 * time.Second,
		WSWriteTimeout:          10 * time.Second,
		CacheSize:               20000,
		CacheTTL:                5 * time.Minute,
		LogLevel:                "info",
		LogFormat:               "json",
		EnableOpenAPIValidation: true,
		CommandsCUEDir:          "auth",
		AuditLogPath:            "/var/log/chit/audit.log",
		WebhookWorkerCount:      4,
		WebhookQueueSize:        1024,
		WebhookTimeoutSec:       10,
	}
}

// Load reads configuration for a process that SERVES chit, and requires
// everything such a process needs — including the database.
//
// Use LoadWithoutDatabase for a tool that does not open one. The distinction is
// not cosmetic: chit-reconcile writes to Keto over HTTP and reads CUE from
// disk, touching no database at all, and requiring a URL of it means handing a
// batch job a Postgres credential it cannot use in order to satisfy a check for
// a field it never reads.
func Load() (*Config, error) {
	cfg, err := load()
	if err != nil {
		return nil, err
	}
	if cfg.DatabaseURL == "" {
		// An ERROR, not log.Fatal. A library that exits the process denies its
		// caller the chance to say which binary failed and why, and this
		// function already returns an error for every other failure.
		return nil, errors.New("CHIT_DATABASE_URL is required")
	}
	return cfg, nil
}

// LoadWithoutDatabase reads the same configuration for a tool that never opens
// the database. Everything else is validated identically.
func LoadWithoutDatabase() (*Config, error) {
	return load()
}

// load reads configuration from environment variables prefixed with CHIT_.
func load() (*Config, error) {
	k := koanf.New(".")
	cfg := Defaults()

	err := k.Load(env.ProviderWithValue("CHIT_", ".", func(key, value string) (string, any) {
		key = strings.ToLower(strings.TrimPrefix(key, "CHIT_"))
		if key == "allowed_origins" {
			parts := strings.Split(value, ",")
			origins := make([]string, 0, len(parts))
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					origins = append(origins, p)
				}
			}
			return key, origins
		}
		return key, value
	}), nil)
	if err != nil {
		return nil, err
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
