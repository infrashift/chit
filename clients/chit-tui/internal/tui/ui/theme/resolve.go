package theme

import (
	"errors"
	"fmt"
	"strings"
)

// Appearance selects between a light and a dark theme when no explicit theme
// is named.
type Appearance int

const (
	// AppearanceUnset means the caller expressed no preference.
	AppearanceUnset Appearance = iota
	AppearanceDark
	AppearanceLight
	// AppearanceSystem asks the terminal which it is using.
	AppearanceSystem
)

func (a Appearance) String() string {
	switch a {
	case AppearanceDark:
		return "dark"
	case AppearanceLight:
		return "light"
	case AppearanceSystem:
		return "system"
	default:
		return ""
	}
}

// ParseAppearance converts a configured string to an Appearance.
func ParseAppearance(s string) (Appearance, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return AppearanceUnset, nil
	case "dark":
		return AppearanceDark, nil
	case "light":
		return AppearanceLight, nil
	case "system":
		return AppearanceSystem, nil
	default:
		return AppearanceUnset, fmt.Errorf(
			`invalid appearance %q: must be "dark", "light", or "system"`, s)
	}
}

// Request describes where a theme choice came from. Splitting the flag from the
// configured value matters because they fail differently: a name typed on the
// command line is a mistake worth stopping for, while a stale config file
// should degrade to a working theme.
type Request struct {
	// FlagTheme is --theme. An unknown name here is fatal.
	FlagTheme string
	// ConfigTheme is the theme setting, already resolved across env and file.
	ConfigTheme string
	// ConfigThemeDark and ConfigThemeLight are chosen between by appearance
	// when no explicit theme is set.
	ConfigThemeDark  string
	ConfigThemeLight string

	FlagAppearance   Appearance
	ConfigAppearance Appearance

	// ThemesDir holds user theme files.
	ThemesDir string

	// SystemIsDark reports whether the terminal is dark. It is injected so
	// resolution stays testable and never probes a terminal on its own; nil
	// is treated as dark.
	SystemIsDark func() bool
}

// Resolve turns a request into a theme, plus any advisory warnings.
//
// Precedence: FlagTheme, then ConfigTheme, then ConfigThemeDark or
// ConfigThemeLight selected by appearance, then the appearance default.
//
// An unknown FlagTheme is an error; an unknown configured theme is a warning
// that falls through to the next source. The asymmetry is deliberate — someone
// who just typed a name wants to know it was wrong, whereas a config file that
// has aged out of date should not stop the client from starting.
func Resolve(req Request) (Theme, []string, error) {
	var warnings []string

	if req.FlagTheme != "" {
		t, w, err := ResolveNamed(req.FlagTheme, req.ThemesDir)
		warnings = append(warnings, w...)
		if err != nil {
			if errors.Is(err, ErrLocalThemeNotFound) {
				return Theme{}, warnings, fmt.Errorf(
					"unknown theme %q. Bundled themes: %s. Local themes are read from %s",
					req.FlagTheme, strings.Join(BuiltinNames(), ", "), req.ThemesDir)
			}
			return Theme{}, warnings, err
		}
		if req.FlagAppearance != AppearanceUnset || req.ConfigAppearance != AppearanceUnset {
			warnings = append(warnings,
				"appearance is ignored because a theme was named explicitly")
		}
		return t, warnings, nil
	}

	if req.ConfigTheme != "" {
		t, w, err := ResolveNamed(req.ConfigTheme, req.ThemesDir)
		warnings = append(warnings, w...)
		if err == nil {
			if req.ConfigAppearance != AppearanceUnset {
				warnings = append(warnings,
					"appearance is ignored because a theme was named explicitly")
			}
			return t, warnings, nil
		}
		warnings = append(warnings, configThemeWarning(req.ConfigTheme, err))
	}

	dark := req.Dark()

	// theme_dark and theme_light name a theme per appearance; only the one
	// matching the resolved appearance is consulted.
	if name := pickByAppearance(req, dark); name != "" {
		t, w, err := ResolveNamed(name, req.ThemesDir)
		warnings = append(warnings, w...)
		if err == nil {
			return t, warnings, nil
		}
		warnings = append(warnings, configThemeWarning(name, err))
	}

	if dark {
		return TokyoNight(), warnings, nil
	}
	return TokyoNightDay(), warnings, nil
}

// Dark reports whether the requested appearance is dark: the flag, then the
// config, then the terminal itself.
func (req Request) Dark() bool {
	appearance := req.FlagAppearance
	if appearance == AppearanceUnset {
		appearance = req.ConfigAppearance
	}
	if appearance == AppearanceUnset {
		appearance = AppearanceSystem
	}
	return appearanceIsDark(appearance, req.SystemIsDark)
}

// pickByAppearance returns the configured theme name for the resolved
// appearance, if one was set.
func pickByAppearance(req Request, dark bool) string {
	if dark {
		return req.ConfigThemeDark
	}
	return req.ConfigThemeLight
}

func appearanceIsDark(a Appearance, systemIsDark func() bool) bool {
	switch a {
	case AppearanceDark:
		return true
	case AppearanceLight:
		return false
	default:
		if systemIsDark == nil {
			return true
		}
		return systemIsDark()
	}
}

// configThemeWarning explains why a configured theme was not used. A missing
// theme and a broken one need different wording: one is a name that resolves
// to nothing, the other is a file that failed to load.
func configThemeWarning(name string, err error) string {
	if errors.Is(err, ErrLocalThemeNotFound) {
		return fmt.Sprintf("unknown theme %q in config, ignoring. Bundled themes: %s",
			name, strings.Join(BuiltinNames(), ", "))
	}
	return fmt.Sprintf("theme %q could not be loaded, ignoring: %v", name, err)
}
