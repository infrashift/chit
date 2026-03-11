package mention

import (
	"regexp"
	"strings"
)

// MentionEntry represents a user or special mention for autocomplete.
type MentionEntry struct {
	Username    string
	DisplayName string
	UserID      string
	Special     bool // true for @all, @channel, @here
}

// MentionSpan represents a mention's position in text.
type MentionSpan struct {
	Start    int
	End      int
	Username string
}

var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_.\-]+)`)

// ExtractMentionPrefix scans backward from cursorCol in text to find an @mention prefix.
// Returns the prefix after @, the column of the @, and whether a mention trigger is active.
func ExtractMentionPrefix(text string, cursorCol int) (prefix string, startCol int, active bool) {
	if cursorCol > len(text) {
		cursorCol = len(text)
	}
	// Scan backward from cursor to find @
	for i := cursorCol - 1; i >= 0; i-- {
		ch := text[i]
		if ch == '@' {
			return text[i+1 : cursorCol], i, true
		}
		if ch == ' ' || ch == '\t' || ch == '\n' {
			return "", 0, false
		}
	}
	return "", 0, false
}

// FilterEntries returns entries whose Username or DisplayName match the prefix (case-insensitive).
// Special mentions are sorted first. If prefix is empty, all entries are returned.
func FilterEntries(entries []MentionEntry, prefix string) []MentionEntry {
	lower := strings.ToLower(prefix)
	var specials, normals []MentionEntry
	for _, e := range entries {
		if lower == "" || strings.HasPrefix(strings.ToLower(e.Username), lower) ||
			strings.HasPrefix(strings.ToLower(e.DisplayName), lower) {
			if e.Special {
				specials = append(specials, e)
			} else {
				normals = append(normals, e)
			}
		}
	}
	return append(specials, normals...)
}

// FindMentions finds all @mention spans in content.
func FindMentions(content string) []MentionSpan {
	matches := mentionRe.FindAllStringSubmatchIndex(content, -1)
	var spans []MentionSpan
	for _, m := range matches {
		spans = append(spans, MentionSpan{
			Start:    m[0],
			End:      m[1],
			Username: content[m[2]:m[3]],
		})
	}
	return spans
}
