package config

import (
	_ "embed"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

//go:embed schema/config.cue
var configSchemaSrc string // semscope: allow(NASA-P10-06) go:embed requires a package variable

//go:embed schema/theme.cue
var themeSchemaSrc string // semscope: allow(NASA-P10-06) go:embed requires a package variable

// maxVetPasses bounds the delete-and-revalidate loop. CUE reports every error
// at once but a unification cannot be partially accepted, so an offending key
// is dropped and the whole document revalidated. The cap is a backstop against
// a schema that somehow never converges.
const maxVetPasses = 64

// keyMessage gives every config key a hand-written explanation. CUE's own
// error text names internal paths and constraint syntax, which is meaningless
// to someone editing a TOML file, so it is never shown. It reports false for a
// key with no message.
func keyMessage(key string) (string, bool) {
	switch key {
	case "server_url", "auth_header", "session_file", "theme", "theme_dark", "theme_light":
		return "must be a string", true
	case "ws_scheme":
		return `must be "ws" or "wss"`, true
	case "appearance":
		return `must be "dark", "light", or "system"`, true
	case "theme_watch", "theme_paint":
		return "must be true or false", true
	default:
		return "", false
	}
}

// compileSchema compiles an embedded schema and returns the named definition.
func compileSchema(src, defName string) (cue.Value, error) {
	assert.That(src != "", "compileSchema: empty schema source")
	assert.That(strings.HasPrefix(defName, "#"), "compileSchema: name is not a definition")
	ctx := cuecontext.New()

	v := ctx.CompileString(src, cue.Filename(strings.TrimPrefix(defName, "#")+".cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, err
	}

	s := v.LookupPath(cue.ParsePath(defName))
	if err := s.Err(); err != nil {
		return cue.Value{}, err
	}
	assert.That(s.Exists(), "compileSchema: definition found without an error but does not exist")
	return s, nil
}

// vetConfig validates decoded TOML against the config schema, dropping keys
// that do not fit and recording a warning for each. It never fails the load:
// a bad key falls back to its default, which keeps a stale config file from
// locking someone out of the client.
func vetConfig(raw map[string]any, warnings *[]string) {
	assert.That(raw != nil, "vetConfig: nil document")
	assert.That(warnings != nil, "vetConfig: nil warnings")
	// Unknown keys are stripped first, and each pass drops one key, so the
	// loop ends long before maxVetPasses.
	assert.That(len(raw) < maxVetPasses, "vetConfig: more keys than passes")
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

		before := len(raw)
		if !dropFirstOffender(raw, verr, warnings) {
			clearMap(raw)
			*warnings = append(*warnings,
				"config: could not validate remaining settings — using defaults")
			return
		}
		// Each pass drops one key, so the loop ends within len(raw) passes.
		assert.That(len(raw) == before-1, "vetConfig: a pass must drop exactly one key")
	}
}

// dropFirstOffender removes the first key CUE complained about and reports
// whether anything was dropped. Returning false means the errors could not be
// traced to a key, so the caller gives up on the document as a whole.
func dropFirstOffender(raw map[string]any, verr error, warnings *[]string) bool {
	assert.That(raw != nil, "dropFirstOffender: nil document")
	assert.That(verr != nil, "dropFirstOffender: no validation error")
	assert.That(warnings != nil, "dropFirstOffender: nil warnings")
	for _, e := range cueerrors.Errors(verr) {
		key := trimDefinitionPrefix(e.Path())
		if key == "" {
			continue
		}
		if _, ok := raw[key]; !ok {
			continue
		}
		assert.That(!strings.HasPrefix(key, "#"), "dropFirstOffender: key still has a definition prefix")
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
	msg, ok := keyMessage(key)
	// A recognized setting is exactly one with an explanation.
	assert.That(ok == (msg != ""), "warnKey: keyMessage must explain each key it knows")
	assert.That(ok == isFileKey(key), "warnKey: keyMessage and fileKeys disagree")
	if ok {
		return fmt.Sprintf("config %q: %s — ignored", key, msg)
	}
	return fmt.Sprintf("config %q: not a recognized setting — ignored", key)
}

func clearMap(m map[string]any) {
	assert.That(m != nil, "clearMap: nil map")
	for k := range m {
		delete(m, k)
	}
	assert.That(len(m) == 0, "clearMap: keys remain")
}
