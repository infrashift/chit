package config

import (
	"slices"
	"testing"
	"time"
)

func TestDefaults_AreValid(t *testing.T) {
	if err := Defaults().validate(); err != nil {
		t.Fatalf("the defaults fail their own validation: %v", err)
	}
}

// The database requirement is the ONLY difference between the two loaders, and
// it is now testable at all: it used to be a log.Fatal inside Load, which exits
// the test binary rather than failing a case.

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("CHIT_DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to reject an empty CHIT_DATABASE_URL")
	}
}

func TestLoadWithoutDatabase_DoesNotRequireDatabaseURL(t *testing.T) {
	t.Setenv("CHIT_DATABASE_URL", "")

	cfg, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatalf("LoadWithoutDatabase with no database URL: %v", err)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected an empty DatabaseURL, got %q", cfg.DatabaseURL)
	}
}

// chit-reconcile reads exactly these two fields, so both loaders must populate
// them the same way. A loader that skipped a validation AND a parse would fix
// the seed job by breaking what it seeds.
func TestBothLoaders_AgreeOnWhatReconcileReads(t *testing.T) {
	t.Setenv("CHIT_DATABASE_URL", "postgres://u:p@127.0.0.1:5432/chit")
	t.Setenv("CHIT_KETO_WRITE_URL", "http://127.0.0.1:14467")
	t.Setenv("CHIT_COMMANDS_CUE_DIR", "auth")

	withDB, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	withoutDB, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatalf("LoadWithoutDatabase: %v", err)
	}

	if withDB.KetoWriteURL != withoutDB.KetoWriteURL {
		t.Fatalf("KetoWriteURL differs: %q vs %q", withDB.KetoWriteURL, withoutDB.KetoWriteURL)
	}
	if withDB.CommandsCUEDir != withoutDB.CommandsCUEDir {
		t.Fatalf("CommandsCUEDir differs: %q vs %q", withDB.CommandsCUEDir, withoutDB.CommandsCUEDir)
	}
	if withoutDB.KetoWriteURL != "http://127.0.0.1:14467" {
		t.Fatalf("KetoWriteURL not read from the environment: %q", withoutDB.KetoWriteURL)
	}
}

func TestLoad_Validation(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		ok   bool
	}{
		{"defaults", nil, true},
		{"a zero ping interval panics the first WebSocket", map[string]string{"CHIT_WS_PING_INTERVAL": "0s"}, false},
		{"a zero cache TTL expires everything", map[string]string{"CHIT_CACHE_TTL": "0s"}, false},
		{"a misspelt pubsub backend", map[string]string{"CHIT_PUBSUB_BACKEND": "pgnotfy"}, false},
		{"nats without a URL", map[string]string{"CHIT_PUBSUB_BACKEND": "nats", "CHIT_NATS_URL": ""}, false},
		{"webhooks on without a URL", map[string]string{"CHIT_WEBHOOK_ENABLED": "true"}, false},
		{"webhooks on, no workers", map[string]string{"CHIT_WEBHOOK_ENABLED": "true", "CHIT_WEBHOOK_URL": "http://h", "CHIT_WEBHOOK_WORKER_COUNT": "0"}, false},
		{"webhooks on and complete", map[string]string{"CHIT_WEBHOOK_ENABLED": "true", "CHIT_WEBHOOK_URL": "http://h"}, true},
		{"more idle than open connections", map[string]string{"CHIT_DB_MAX_IDLE_CONN": "30"}, false},
		{"an unknown log level", map[string]string{"CHIT_LOG_LEVEL": "verbose"}, false},
		{"an unparseable duration", map[string]string{"CHIT_WS_WRITE_TIMEOUT": "soon"}, false},
		{"S3 without credentials", map[string]string{"CHIT_S3_ENDPOINT": "http://localhost:8333"}, false},
		{"S3 complete", map[string]string{"CHIT_S3_ENDPOINT": "http://localhost:8333",
			"CHIT_S3_ACCESS_KEY_ID": "a", "CHIT_S3_SECRET_ACCESS_KEY": "s"}, true},
		{"S3 with a zero quota", map[string]string{"CHIT_S3_ENDPOINT": "http://localhost:8333",
			"CHIT_S3_ACCESS_KEY_ID": "a", "CHIT_S3_SECRET_ACCESS_KEY": "s", "CHIT_FILE_QUOTA_BYTES": "0"}, false},
		{"S3 with a zero grace", map[string]string{"CHIT_S3_ENDPOINT": "http://localhost:8333",
			"CHIT_S3_ACCESS_KEY_ID": "a", "CHIT_S3_SECRET_ACCESS_KEY": "s", "CHIT_FILE_ORPHAN_GRACE": "0s"}, false},
		{"membership hook without a token URL", map[string]string{"CHIT_MEMBERSHIP_HOOK_URL": "http://h/check",
			"CHIT_MEMBERSHIP_HOOK_CLIENT_ID": "chit", "CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET": "s"}, false},
		{"membership hook without a client ID", map[string]string{"CHIT_MEMBERSHIP_HOOK_URL": "http://h/check",
			"CHIT_MEMBERSHIP_HOOK_TOKEN_URL": "http://t", "CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET": "s"}, false},
		{"membership hook without a client secret", map[string]string{"CHIT_MEMBERSHIP_HOOK_URL": "http://h/check",
			"CHIT_MEMBERSHIP_HOOK_TOKEN_URL": "http://t", "CHIT_MEMBERSHIP_HOOK_CLIENT_ID": "chit"}, false},
		{"membership hook with a zero timeout", map[string]string{"CHIT_MEMBERSHIP_HOOK_URL": "http://h/check",
			"CHIT_MEMBERSHIP_HOOK_TOKEN_URL": "http://t", "CHIT_MEMBERSHIP_HOOK_CLIENT_ID": "chit",
			"CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET": "s", "CHIT_MEMBERSHIP_HOOK_TIMEOUT": "0s"}, false},
		{"membership hook complete", map[string]string{"CHIT_MEMBERSHIP_HOOK_URL": "http://h/check",
			"CHIT_MEMBERSHIP_HOOK_TOKEN_URL": "http://t", "CHIT_MEMBERSHIP_HOOK_CLIENT_ID": "chit",
			"CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET": "s"}, true},
		{"membership hook credentials alone do nothing", map[string]string{"CHIT_MEMBERSHIP_HOOK_CLIENT_ID": "chit",
			"CHIT_MEMBERSHIP_HOOK_TIMEOUT": "0s"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := LoadWithoutDatabase()
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestLoad_AllowedOrigins(t *testing.T) {
	cases := map[string][]string{
		"https://a.example, https://b.example ,": {"https://a.example", "https://b.example"},
		"*":                                      {"*"},
		"":                                       {},
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("CHIT_ALLOWED_ORIGINS", raw)
			cfg, err := LoadWithoutDatabase()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.AllowedOrigins, want) {
				t.Fatalf("got %q, want %q", cfg.AllowedOrigins, want)
			}
		})
	}
}

func TestLoad_Durations(t *testing.T) {
	t.Setenv("CHIT_WS_PING_INTERVAL", "45s")
	t.Setenv("CHIT_CACHE_TTL", "2m")
	cfg, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WSPingInterval != 45*time.Second || cfg.CacheTTL != 2*time.Minute {
		t.Fatalf("ping=%v ttl=%v", cfg.WSPingInterval, cfg.CacheTTL)
	}
}

// The membership hook is off by default, and its timeout is 5 seconds.
func TestLoad_MembershipHook(t *testing.T) {
	cfg, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MembershipHookURL != "" || cfg.MembershipHookTimeout != 5*time.Second {
		t.Fatalf("defaults: url=%q timeout=%v", cfg.MembershipHookURL, cfg.MembershipHookTimeout)
	}
	t.Setenv("CHIT_MEMBERSHIP_HOOK_URL", "http://halod:8080/internal/chit/membership-check")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_TIMEOUT", "750ms")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_TOKEN_URL", "http://hydra:4444/oauth2/token")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_CLIENT_ID", "chit")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_CLIENT_SECRET", "secret")
	t.Setenv("CHIT_MEMBERSHIP_HOOK_SCOPE", "halo:membership")
	if cfg, err = LoadWithoutDatabase(); err != nil {
		t.Fatal(err)
	}
	if cfg.MembershipHookURL != "http://halod:8080/internal/chit/membership-check" ||
		cfg.MembershipHookTimeout != 750*time.Millisecond ||
		cfg.MembershipHookTokenURL != "http://hydra:4444/oauth2/token" ||
		cfg.MembershipHookClientID != "chit" || cfg.MembershipHookClientSecret != "secret" ||
		cfg.MembershipHookScope != "halo:membership" {
		t.Fatalf("not read from the environment: %+v", cfg)
	}
}

// CHIT_ON_BEHALF_OF_CLIENTS is a comma-separated list, like the origins, and
// empty by default: no client acts on behalf of anyone unless named.
func TestLoad_OnBehalfOfClients(t *testing.T) {
	cfg, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.OnBehalfOfClients) != 0 {
		t.Fatalf("default %q, want none", cfg.OnBehalfOfClients)
	}
	t.Setenv("CHIT_ON_BEHALF_OF_CLIENTS", " halod , ,other")
	if cfg, err = LoadWithoutDatabase(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"halod", "other"}; !slices.Equal(cfg.OnBehalfOfClients, want) {
		t.Fatalf("got %q, want %q", cfg.OnBehalfOfClients, want)
	}
}

// The per-user limit keeps the values chitd always had; the aggregate limit
// of an on-behalf-of client is sized for HALO (see Defaults).
func TestLoad_RateLimits(t *testing.T) {
	cfg, err := LoadWithoutDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimitUserPerSecond != 10 || cfg.RateLimitUserBurst != 50 ||
		cfg.RateLimitClientPerSecond != 1500 || cfg.RateLimitClientBurst != 3000 {
		t.Fatalf("defaults: user %v/%d, client %v/%d", cfg.RateLimitUserPerSecond, cfg.RateLimitUserBurst,
			cfg.RateLimitClientPerSecond, cfg.RateLimitClientBurst)
	}
	t.Setenv("CHIT_RATE_LIMIT_USER_PER_SECOND", "2.5")
	t.Setenv("CHIT_RATE_LIMIT_USER_BURST", "7")
	t.Setenv("CHIT_RATE_LIMIT_CLIENT_PER_SECOND", "400")
	t.Setenv("CHIT_RATE_LIMIT_CLIENT_BURST", "900")
	if cfg, err = LoadWithoutDatabase(); err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimitUserPerSecond != 2.5 || cfg.RateLimitUserBurst != 7 ||
		cfg.RateLimitClientPerSecond != 400 || cfg.RateLimitClientBurst != 900 {
		t.Fatalf("not read from the environment: user %v/%d, client %v/%d", cfg.RateLimitUserPerSecond,
			cfg.RateLimitUserBurst, cfg.RateLimitClientPerSecond, cfg.RateLimitClientBurst)
	}
}

// Every limit must be a real one: no zero (which refuses everything), no
// "off", and nothing past the bounds.
func TestLoad_RateLimitBounds(t *testing.T) {
	type bound struct {
		value string
		ok    bool
	}
	rates := []bound{
		{"0.001", true}, {"1", true}, {"100000", true},
		{"0", false}, {"-1", false}, {"100000.5", false}, {"NaN", false}, {"Inf", false}, {"-Inf", false},
		{"fast", false}, {"", false},
	}
	bursts := []bound{
		{"1", true}, {"1000000", true},
		{"0", false}, {"-5", false}, {"1000001", false}, {"2.5", false}, {"many", false},
	}
	for _, name := range []string{"CHIT_RATE_LIMIT_USER_PER_SECOND", "CHIT_RATE_LIMIT_CLIENT_PER_SECOND"} {
		for _, b := range rates {
			t.Run(name+"="+b.value, func(t *testing.T) {
				t.Setenv(name, b.value)
				if _, err := LoadWithoutDatabase(); (err == nil) != b.ok {
					t.Fatalf("err = %v, want ok=%v", err, b.ok)
				}
			})
		}
	}
	for _, name := range []string{"CHIT_RATE_LIMIT_USER_BURST", "CHIT_RATE_LIMIT_CLIENT_BURST"} {
		for _, b := range bursts {
			t.Run(name+"="+b.value, func(t *testing.T) {
				t.Setenv(name, b.value)
				if _, err := LoadWithoutDatabase(); (err == nil) != b.ok {
					t.Fatalf("err = %v, want ok=%v", err, b.ok)
				}
			})
		}
	}
}
