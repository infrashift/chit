package config

import (
	_ "embed"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
)

//go:embed schema/config.cue
var configSchemaSrc string

//go:embed schema/theme.cue
var themeSchemaSrc string

// maxVetPasses bounds the delete-and-revalidate loop. CUE reports every error
// at once but a unification cannot be partially accepted, so an offending key
// is dropped and the whole document revalidated. The cap is a backstop against
// a schema that somehow never converges.
const maxVetPasses = 64

// keyMessages gives every config key a hand-written explanation. CUE's own
// error text names internal paths and constraint syntax, which is meaningless
// to someone editing a TOML file, so it is never shown.
var keyMessages = map[string]string{
	"server_url":   "must be a string",
	"ws_scheme":    `must be "ws" or "wss"`,
	"auth_header":  "must be a string",
	"session_file": "must be a string",
	"theme":        "must be a string",
	"theme_dark":   "must be a string",
	"theme_light":  "must be a string",
	"appearance":   `must be "dark", "light", or "system"`,
}

// compileSchema compiles an embedded schema and returns the named definition.
func compileSchema(src, defName string) (cue.Value, error) {
	ctx := cuecontext.New()

	v := ctx.CompileString(src, cue.Filename(strings.TrimPrefix(defName, "#")+".cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, err
	}

	s := v.LookupPath(cue.ParsePath(defName))
	if err := s.Err(); err != nil {
		return cue.Value{}, err
	}
	return s, nil
}

// vetConfig validates decoded TOML against the config schema, dropping keys
// that do not fit and recording a warning for each. It never fails the load:
// a bad key falls back to its default, which keeps a stale config file from
// locking someone out of the client.
func vetConfig(raw map[string]any, warnings *[]string) {
	schema, err := compileSchema(configSchemaSrc, "#Config")
	if err != nil {
		*warnings = append(*warnings,
			fmt.Sprintf("config: schema failed to compile (%v) — skipping validation", err))
		return
	}

	ctx := cuecontext.New()
	for pass := 0; pass < maxVetPasses; pass++ {
		data := ctx.Encode(raw)
		if err := data.Err(); err != nil {
			clearMap(raw)
			*warnings = append(*warnings,
				fmt.Sprintf("config: could not be read (%v) — using defaults", err))
			return
		}

		verr := schema.Unify(data).Validate(cue.Final(), cue.Concrete(true))
		if verr == nil {
			return
		}

		if !dropFirstOffender(raw, verr, warnings) {
			clearMap(raw)
			*warnings = append(*warnings,
				"config: could not validate remaining settings — using defaults")
			return
		}
	}
}

// dropFirstOffender removes the first key CUE complained about and reports
// whether anything was dropped. Returning false means the errors could not be
// traced to a key, so the caller gives up on the document as a whole.
func dropFirstOffender(raw map[string]any, verr error, warnings *[]string) bool {
	for _, e := range cueerrors.Errors(verr) {
		key := trimDefinitionPrefix(e.Path())
		if key == "" {
			continue
		}
		if _, ok := raw[key]; !ok {
			continue
		}
		delete(raw, key)
		*warnings = append(*warnings, warnKey(key))
		return true
	}
	return false
}

// trimDefinitionPrefix turns a CUE error path into a top-level TOML key. CUE
// prefixes paths with the definition selector (#Config), which is an internal
// detail of how the schema is written.
func trimDefinitionPrefix(path []string) string {
	for _, seg := range path {
		if strings.HasPrefix(seg, "#") {
			continue
		}
		return seg
	}
	return ""
}

// warnKey renders the user-facing message for a rejected key.
func warnKey(key string) string {
	if msg, ok := keyMessages[key]; ok {
		return fmt.Sprintf("config %q: %s — ignored", key, msg)
	}
	return fmt.Sprintf("config %q: not a recognized setting — ignored", key)
}

func clearMap(m map[string]any) {
	for k := range m {
		delete(m, k)
	}
}
