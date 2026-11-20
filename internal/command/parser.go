package command

import (
	"regexp"
	"strings"

	"github.com/infrashift/chit/internal/assert"
)

// semscope: allow(NASA-P10-06) compiled once, never changed
var slashCmdRe = regexp.MustCompile(`^/([a-z0-9-]+)(\s+.*)?$`)

// ParseResult holds the parsed slug and arguments from a slash command.
type ParseResult struct {
	Slug string
	Args string
}

// Parse extracts a slash command slug and optional arguments from content.
// It returns false if the content is not a valid slash command.
func Parse(content string) (ParseResult, bool) {
	m := slashCmdRe.FindStringSubmatch(content)
	if m == nil {
		return ParseResult{}, false
	}
	assert.That(len(m) == 3, "Parse: match without its two groups")
	args := ""
	if len(m) > 2 && m[2] != "" {
		assert.That(strings.ContainsRune(" \t\n\f\r\v", rune(m[2][0])), "Parse: args group does not start with white space")
		// Trim leading whitespace captured by the regex group.
		args = m[2][1:] // strip the leading space
	}
	assert.That(m[1] != "", "Parse: empty slug")
	return ParseResult{Slug: m[1], Args: args}, true
}
