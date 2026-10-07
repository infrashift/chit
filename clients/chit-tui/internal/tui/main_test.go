package tui_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui"
)

// TestMain moves every user directory into a temporary one. The model writes
// to the real ones in the normal course of things: picking a skin saves the
// theme to ~/.config/chit/config.toml, and signing out or expiring removes
// ~/.config/chit-tui/session.json. Before this, running the suite did both to
// whoever ran it.
//
// It is set here rather than per test so tests added later are covered too.
func TestMain(m *testing.M) {
	// Split so the deferred cleanup actually runs; os.Exit would skip it.
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "chit-tui-test")
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
	tui.SetClipboardOutput(io.Discard)

	return m.Run()
}
