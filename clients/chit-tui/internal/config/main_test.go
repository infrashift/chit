package config_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points CHIT_CONFIG_FILE at a path that does not exist, so the suite
// never reads the developer's real ~/.config/chit/config.toml. Without this,
// results would depend on whoever is running the tests — a machine with a
// server_url in its config would quietly pass TestLoad_MissingServerURL.
//
// It is set here rather than per-test via t.Setenv so it also covers tests
// added later, which would otherwise silently lose the isolation.
func TestMain(m *testing.M) {
	// Split so the deferred cleanup actually runs; os.Exit would skip it.
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "chit-config-test")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if err := os.Setenv("CHIT_CONFIG_FILE", filepath.Join(dir, "absent.toml")); err != nil {
		panic(err)
	}

	return m.Run()
}
