package config

import (
	"errors"
	"fmt"
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
		ListenAddress:       ":8065",
		DBMaxOpenConn:       25,
		DBMaxIdleConn:       10,
		PubSubBackend:       "pgnotify",
		NatsURL:             "nats://localhost:4222",
		TrustedProxyHeader:  "X-User-Id",
		TrustedClientHeader: "X-Client-Id",
		AllowedOrigins:      []string{"*"},
		KratosAdminURL:      "http://localhost:4434",
		KetoReadURL:         "http://localhost:4466",
		KetoWriteURL:        "http://localhost:4467",
		// Empty: ZincSearch is opt-in. Search then uses SQL, and no indexer
		// runs. The deploy manifests set it.
		ZincSearchURL:           "",
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
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate refuses settings that would fail later and less legibly: a zero
// ping interval panics on the first WebSocket connection (time.NewTicker), a
// zero cache TTL expires every entry at once, and a misspelt pubsub backend
// silently fell back to pgnotify.
func (c *Config) validate() error {
	var errs []error
	positive := func(name string, v int64) {
		if v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be positive, got %d", name, v))
		}
	}
	positive("CHIT_WS_PING_INTERVAL", int64(c.WSPingInterval))
	positive("CHIT_WS_WRITE_TIMEOUT", int64(c.WSWriteTimeout))
	positive("CHIT_CACHE_SIZE", int64(c.CacheSize))
	positive("CHIT_CACHE_TTL", int64(c.CacheTTL))
	positive("CHIT_DB_MAX_OPEN_CONN", int64(c.DBMaxOpenConn))
	if c.DBMaxIdleConn < 0 || c.DBMaxIdleConn > c.DBMaxOpenConn {
		errs = append(errs, fmt.Errorf("CHIT_DB_MAX_IDLE_CONN must be between 0 and CHIT_DB_MAX_OPEN_CONN (%d), got %d",
			c.DBMaxOpenConn, c.DBMaxIdleConn))
	}

	switch c.PubSubBackend {
	case "pgnotify":
	case "nats":
		if c.NatsURL == "" {
			errs = append(errs, errors.New("CHIT_NATS_URL is required when CHIT_PUBSUB_BACKEND=nats"))
		}
	default:
		errs = append(errs, fmt.Errorf("CHIT_PUBSUB_BACKEND must be pgnotify or nats, got %q", c.PubSubBackend))
	}

	if c.TrustedProxyHeader == "" {
		errs = append(errs, errors.New("CHIT_TRUSTED_PROXY_HEADER must not be empty"))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("CHIT_LOG_LEVEL must be debug, info, warn or error, got %q", c.LogLevel))
	}
	switch c.LogFormat {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("CHIT_LOG_FORMAT must be json or text, got %q", c.LogFormat))
	}

	if c.WebhookEnabled {
		// Enabled with no URL used to be silently disabled.
		if c.WebhookURL == "" {
			errs = append(errs, errors.New("CHIT_WEBHOOK_URL is required when CHIT_WEBHOOK_ENABLED=true"))
		}
		positive("CHIT_WEBHOOK_WORKER_COUNT", int64(c.WebhookWorkerCount))
		positive("CHIT_WEBHOOK_QUEUE_SIZE", int64(c.WebhookQueueSize))
		positive("CHIT_WEBHOOK_TIMEOUT_SEC", int64(c.WebhookTimeoutSec))
	}
	return errors.Join(errs...)
}
