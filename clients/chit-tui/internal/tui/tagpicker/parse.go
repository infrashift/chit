package tagpicker

import (
	"regexp"
	"strings"
)

// hashtagRe matches a #tag at the start of a line or after a space or tab.
// A tag needs a letter, so "#123" stays an issue or PR reference.
var hashtagRe = regexp.MustCompile(`(^|[ \t])#([a-zA-Z0-9_-]*[a-zA-Z][a-zA-Z0-9_-]*)`)

// ExtractHashtags returns tag names found in content, lowercased and
// deduplicated. Code, fenced or inline, is quoted verbatim and never holds
// tags: "#include" in a C snippet is not one.
func ExtractHashtags(content string) []string {
	_, tags := scanHashtags(content, false)
	return tags
}

// StripHashtags removes #tag tokens from content, returning cleaned content
// and the list of extracted tag names. Only the tags go, each with the one
// space that separated it; indentation, runs of spaces and line breaks are
// left as written, and a line that held nothing but tags is dropped.
func StripHashtags(content string) (string, []string) {
	return scanHashtags(content, true)
}

func scanHashtags(content string, strip bool) (string, []string) {
	var tags []string
	seen := make(map[string]bool)
	inFence := false
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}

		// Backticks alternate text and inline code: even parts are text.
		// An unmatched final backtick opens no span, so the part after it
		// is text as well.
		parts := strings.Split(line, "`")
		removed := false
		for i := range parts {
			unmatched := i == len(parts)-1 && len(parts)%2 == 0
			if i%2 == 1 && !unmatched {
				continue
			}
			var kept strings.Builder
			last, skipSpace := 0, false
			for _, mt := range hashtagRe.FindAllStringSubmatchIndex(parts[i], -1) {
				start, end, lead := mt[0], mt[1], mt[3]-mt[2]
				// "^" is the start of the segment; only at the start of the
				// line does it mean a tag, not text right after inline code.
				if lead == 0 && i > 0 {
					continue
				}
				name := strings.ToLower(parts[i][mt[4]:mt[5]])
				if !seen[name] {
					seen[name] = true
					tags = append(tags, name)
				}
				if !strip {
					continue
				}
				between := parts[i][last:start]
				if skipSpace {
					between = trimOneSpace(between)
				}
				kept.WriteString(between)
				last = end
				// A tag that opened the line takes the space after it.
				skipSpace = lead == 0
				removed = true
			}
			if strip {
				rest := parts[i][last:]
				if skipSpace {
					rest = trimOneSpace(rest)
				}
				kept.WriteString(rest)
				parts[i] = kept.String()
			}
		}

		if !removed {
			out = append(out, line)
			continue
		}
		line = strings.TrimRight(strings.Join(parts, "`"), " \t")
		if line != "" {
			out = append(out, line)
		}
	}

	if !strip {
		return content, tags
	}
	return strings.Trim(strings.Join(out, "\n"), "\n"), tags
}

func trimOneSpace(s string) string {
	if s != "" && (s[0] == ' ' || s[0] == '\t') {
		return s[1:]
	}
	return s
}
