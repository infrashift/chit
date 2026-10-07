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
