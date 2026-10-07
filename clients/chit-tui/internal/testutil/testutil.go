// Package testutil holds helpers shared by the test suites.
package testutil

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// StripANSI removes every terminal escape sequence from s: colors, cursor
// movement, and OSC sequences such as hyperlinks, not only SGR colors.
func StripANSI(s string) string { return ansi.Strip(s) }

// Styles returns the default styles, for components that need some.
func Styles() styles.Styles { return styles.New(theme.TokyoNight()) }
