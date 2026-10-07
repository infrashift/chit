package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

// TestMain points every user directory at a temporary one, so listing and
// resolving themes never reads the developer's own config or theme files.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "chit-tui-cmd-test")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	for k, v := range map[string]string{
		"HOME":             dir,
		"XDG_CONFIG_HOME":  filepath.Join(dir, "config"),
		"CHIT_CONFIG_FILE": filepath.Join(dir, "config", "chit", "config.toml"),
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	// xdg resolves its directories once, at init, before TestMain runs.
	xdg.Reload()
	return m.Run()
}
