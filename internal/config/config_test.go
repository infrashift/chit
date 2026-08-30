package config

import "testing"

func TestDefaults_EnableOpenAPIValidation(t *testing.T) {
	cfg := Defaults()
	if !cfg.EnableOpenAPIValidation {
		t.Fatal("expected EnableOpenAPIValidation to default to true")
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
