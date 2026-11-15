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
	// OnBehalfOfClients lists the OAuth2 client IDs (comma-separated in
	// CHIT_ON_BEHALF_OF_CLIENTS) that may act as a person by naming them in
	// X-On-Behalf-Of. Each must be a machine actor in MachineActors, that is a
	// Hydra client_credentials client. Empty: no client may.
	OnBehalfOfClients []string `koanf:"on_behalf_of_clients"`
	// Rate limits (P-CHIT-8). Each user has a bucket of RateLimitUserPerSecond
	// requests a second, in bursts of RateLimitUserBurst. A client in
	// OnBehalfOfClients acts for many people, so it has a bucket of its own
	// instead, RateLimitClientPerSecond in bursts of RateLimitClientBurst,
	// that counts everything it does: as itself and on behalf of anyone. A
	// request on behalf of a person also counts against that person's bucket.
	RateLimitUserPerSecond   float64 `koanf:"rate_limit_user_per_second"`
	RateLimitUserBurst       int     `koanf:"rate_limit_user_burst"`
	RateLimitClientPerSecond float64 `koanf:"rate_limit_client_per_second"`
	RateLimitClientBurst     int     `koanf:"rate_limit_client_burst"`
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

	// Attachments (P-CHIT-5). With S3Endpoint empty, attachments are off:
	// the file routes answer 503 and a post with file_ids is a 400.
	// S3Endpoint is an S3-compatible endpoint (SeaweedFS), addressed
	// path-style; chitd creates S3Bucket at start when it is missing.
	S3Endpoint        string `koanf:"s3_endpoint"`
	S3Bucket          string `koanf:"s3_bucket"`
	S3Region          string `koanf:"s3_region"`
	S3AccessKeyID     string `koanf:"s3_access_key_id"`
	S3SecretAccessKey string `koanf:"s3_secret_access_key"`
	// FileQuotaBytes bounds the live attachment bytes of one person.
	FileQuotaBytes int64 `koanf:"file_quota_bytes"`
	// FileOrphanGrace is how long an upload may wait for a post before the
	// sweep deletes it, and how long a deleted file's object is kept.
	FileOrphanGrace time.Duration `koanf:"file_orphan_grace"`
	// FileSweepInterval and FileSweepBatch bound the sweep: at most
	// FileSweepBatch files each FileSweepInterval.
	FileSweepInterval time.Duration `koanf:"file_sweep_interval"`
	FileSweepBatch    int           `koanf:"file_sweep_batch"`

	// Membership policy hook (P-CHIT-2). With MembershipHookURL empty the
	// hook is off and members are added as before. With it set, chitd POSTs
	// each new member to the URL before it adds them, and refuses the add
	// unless the answer is {"allow":true}. It authenticates with a Hydra
	// client_credentials token of its own OAuth2 client.
	MembershipHookURL          string        `koanf:"membership_hook_url"`
	MembershipHookTimeout      time.Duration `koanf:"membership_hook_timeout"`
	MembershipHookTokenURL     string        `koanf:"membership_hook_token_url"`
	MembershipHookClientID     string        `koanf:"membership_hook_client_id"`
	MembershipHookClientSecret string        `koanf:"membership_hook_client_secret"`
	// MembershipHookScope is optional: space-separated OAuth2 scopes.
	MembershipHookScope string `koanf:"membership_hook_scope"`

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
		// The per-user limit chitd has always had.
		RateLimitUserPerSecond: 10,
		RateLimitUserBurst:     50,
		// Sized for HALO's halod with about 2000 Subjects at about one
		// Message a minute each, with streamed replies: about 1000 requests
		// a second, so 1500 leaves half again as much. The burst is two
		// seconds of it. See the configuration reference for the arithmetic.
		RateLimitClientPerSecond: 1500,
		RateLimitClientBurst:     3000,
		KratosAdminURL:           "http://localhost:4434",
		KetoReadURL:              "http://localhost:4466",
		KetoWriteURL:             "http://localhost:4467",
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
		S3Bucket:                "chit-files",
		S3Region:                "us-east-1",
		FileQuotaBytes:          1 << 30,
		FileOrphanGrace:         24 * time.Hour,
		FileSweepInterval:       10 * time.Minute,
		FileSweepBatch:          100,
		MembershipHookTimeout:   5 * time.Second,
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
		if key == "allowed_origins" || key == "on_behalf_of_clients" {
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

// Bounds of the rate limits. A limit of 0 would refuse every request, and
// there is no setting that turns the limit off: a limit above these is a
// typo, not a deployment (one chitd serves well under 100,000 requests a
// second).
const (
	MaxRateLimitPerSecond = 100_000
	MaxRateLimitBurst     = 1_000_000
)

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

	rateLimit := func(name string, perSecond float64, burstName string, burst int) {
		// Written so that NaN fails too: every comparison with NaN is false.
		if !(perSecond > 0 && perSecond <= MaxRateLimitPerSecond) {
			errs = append(errs, fmt.Errorf("%s must be more than 0 and at most %d, got %v",
				name, MaxRateLimitPerSecond, perSecond))
		}
		if burst < 1 || burst > MaxRateLimitBurst {
			errs = append(errs, fmt.Errorf("%s must be between 1 and %d, got %d", burstName, MaxRateLimitBurst, burst))
		}
	}
	rateLimit("CHIT_RATE_LIMIT_USER_PER_SECOND", c.RateLimitUserPerSecond, "CHIT_RATE_LIMIT_USER_BURST", c.RateLimitUserBurst)
	rateLimit("CHIT_RATE_LIMIT_CLIENT_PER_SECOND", c.RateLimitClientPerSecond, "CHIT_RATE_LIMIT_CLIENT_BURST", c.RateLimitClientBurst)

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

	if c.S3Endpoint != "" {
		if c.S3Bucket == "" || c.S3Region == "" || c.S3AccessKeyID == "" || c.S3SecretAccessKey == "" {
			errs = append(errs, errors.New("CHIT_S3_BUCKET, CHIT_S3_REGION, CHIT_S3_ACCESS_KEY_ID and "+
				"CHIT_S3_SECRET_ACCESS_KEY are required when CHIT_S3_ENDPOINT is set"))
		}
		positive("CHIT_FILE_QUOTA_BYTES", c.FileQuotaBytes)
		positive("CHIT_FILE_ORPHAN_GRACE", int64(c.FileOrphanGrace))
		positive("CHIT_FILE_SWEEP_INTERVAL", int64(c.FileSweepInterval))
		positive("CHIT_FILE_SWEEP_BATCH", int64(c.FileSweepBatch))
	}

	if c.MembershipHookURL != "" {
		if c.MembershipHookTokenURL == "" || c.MembershipHookClientID == "" || c.MembershipHookClientSecret == "" {
			errs = append(errs, errors.New("CHIT_MEMBERSHIP_HOOK_TOKEN_URL, CHIT_MEMBERSHIP_HOOK_CLIENT_ID and "+
				"CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET are required when CHIT_MEMBERSHIP_HOOK_URL is set"))
		}
		positive("CHIT_MEMBERSHIP_HOOK_TIMEOUT", int64(c.MembershipHookTimeout))
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
