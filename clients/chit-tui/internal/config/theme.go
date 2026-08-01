package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"

	"github.com/BurntSushi/toml"
)

// ThemeColorKeys is the canonical palette key list for a theme file. It must
// stay in step with the Theme struct, the #Theme definition in
// schema/theme.cue, and the loader that reads these keys back out.
// TestThemeColorKeysMatchSchema guards that correspondence.
var ThemeColorKeys = []string{
	"background",
	"foreground",
	"subtle",
	"accent",
	"error",
	"success",
	"warning",
	"border",
	"active_border",
	"highlight",
	"muted",
	"username",
	"timestamp",
	"unread_badge",
	"pin_badge",
	"channel_active",
	"mention_badge",
	"mention_text",
	"mention_self_bg",
	"tag_badge",
}

// ThemeDerivedKeys are optional; when a file omits them they are computed from
// the palette.
var ThemeDerivedKeys = []string{"selection", "search_match", "search_match_active"}

// ThemeMeta are the non-color keys.
var ThemeMeta = []string{"name", "author"}

// ErrThemeNotFound reports that no theme file exists at the given path, which
// callers distinguish from a file that exists but is broken.
var ErrThemeNotFound = errors.New("theme file not found")

// ValidateThemeTOML reads a theme file and returns its keys as raw strings for
// the theme package to parse into colors. Unlike a config file, a broken theme
// is an error rather than a warning: silently substituting defaults would leave
// the user staring at a theme that is not the one they asked for.
func ValidateThemeTOML(path string) (values map[string]string, warnings []string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrThemeNotFound
		}
		return nil, nil, fmt.Errorf("read theme %s: %w", path, err)
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, nil, fmt.Errorf("theme %s: invalid TOML: %w", path, err)
	}

	warnings = stripUnknownThemeKeys(raw)

	// Missing required keys are detected here rather than left to CUE: when a
	// document also has value errors, CUE suppresses its required-field
	// diagnostics, so a file with one bad color and three absent ones would
	// report only the bad one and the user would fix them one run at a time.
	var problems []string
	problems = append(problems, missingThemeKeys(raw)...)
	problems = append(problems, validateThemeValues(raw)...)

	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, warnings, fmt.Errorf("theme %s:\n  %s", path, strings.Join(problems, "\n  "))
	}

	values = make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			values[k] = s
		}
	}
	return values, warnings, nil
}

func knownThemeKeys() map[string]bool {
	known := make(map[string]bool, len(ThemeColorKeys)+len(ThemeDerivedKeys)+len(ThemeMeta))
	for _, k := range ThemeColorKeys {
		known[k] = true
	}
	for _, k := range ThemeDerivedKeys {
		known[k] = true
	}
	for _, k := range ThemeMeta {
		known[k] = true
	}
	return known
}

// stripUnknownThemeKeys drops keys the schema does not define. An unrecognized
// key is a warning, not an error — it is most likely a stale key from an older
// version, and refusing to load over it would be unhelpful.
func stripUnknownThemeKeys(raw map[string]any) []string {
	known := knownThemeKeys()

	unknown := make([]string, 0)
	for k := range raw {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)

	warnings := make([]string, 0, len(unknown))
	for _, k := range unknown {
		delete(raw, k)
		warnings = append(warnings, fmt.Sprintf("theme %q: not a recognized key — ignored", k))
	}
	return warnings
}

func missingThemeKeys(raw map[string]any) []string {
	var missing []string
	if _, ok := raw["name"]; !ok {
		missing = append(missing, "key `name` is required")
	}
	for _, k := range ThemeColorKeys {
		if _, ok := raw[k]; !ok {
			missing = append(missing, fmt.Sprintf("key `%s` is required", k))
		}
	}
	return missing
}

// validateThemeValues unifies the document with the CUE schema and translates
// each rejected key into a message written for someone editing a TOML file.
func validateThemeValues(raw map[string]any) []string {
	schema, err := compileSchema(themeSchemaSrc, "#Theme")
	if err != nil {
		return []string{fmt.Sprintf("theme schema failed to compile: %v", err)}
	}

	encoded := cuecontext.New().Encode(raw)
	if err := encoded.Err(); err != nil {
		return []string{fmt.Sprintf("theme could not be read: %v", err)}
	}

	verr := schema.Unify(encoded).Validate(cue.All(), cue.Concrete(true))
	if verr == nil {
		return nil
	}
	return themeErrorMessages(raw, verr)
}

// themeErrorMessages renders one message per offending key, deduplicated —
// CUE frequently reports the same key from several constraint branches.
func themeErrorMessages(raw map[string]any, verr error) []string {
	colorKeys := make(map[string]bool, len(ThemeColorKeys)+len(ThemeDerivedKeys))
	for _, k := range ThemeColorKeys {
		colorKeys[k] = true
	}
	for _, k := range ThemeDerivedKeys {
		colorKeys[k] = true
	}

	seen := make(map[string]bool)
	var msgs []string

	for _, e := range cueerrors.Errors(verr) {
		key := trimDefinitionPrefix(e.Path())
		if key == "" || seen[key] {
			continue
		}
		// A key reported as missing is already covered by missingThemeKeys.
		if _, present := raw[key]; !present {
			continue
		}
		seen[key] = true

		switch {
		case colorKeys[key]:
			msgs = append(msgs, fmt.Sprintf("key `%s` must be a hex color such as #7aa2f7", key))
		case key == "name" || key == "author":
			msgs = append(msgs, fmt.Sprintf("key `%s` must be a string", key))
		default:
			msgs = append(msgs, fmt.Sprintf("key `%s` is not valid", key))
		}
	}
	return msgs
}
