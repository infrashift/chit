package command

import "regexp"

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
	args := ""
	if len(m) > 2 && m[2] != "" {
		// Trim leading whitespace captured by the regex group.
		args = m[2][1:] // strip the leading space
	}
	return ParseResult{Slug: m[1], Args: args}, true
}
