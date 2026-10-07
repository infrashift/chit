package mention

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
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

// mentionRe matches @name. A name may contain dots and dashes but not end in
// one, so the full stop closing "thanks @bob." is not part of the name.
var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_](?:[a-zA-Z0-9_.\-]*[a-zA-Z0-9_])?)`)

// ExtractMentionPrefix scans backward from cursorCol in text to find an @mention prefix.
// Returns the prefix after @, the column of the @, and whether a mention trigger is active.
// Columns count characters, not bytes, as the input's cursor does. An @
// inside a word is an email address and does not trigger.
func ExtractMentionPrefix(text string, cursorCol int) (prefix string, startCol int, active bool) {
	runes := []rune(text)
	cursorCol = min(cursorCol, len(runes))
	for i := cursorCol - 1; i >= 0; i-- {
		switch r := runes[i]; {
		case r == '@':
			if i > 0 && isNameRune(runes[i-1]) {
				return "", 0, false
			}
			return string(runes[i+1 : cursorCol]), i, true
		case unicode.IsSpace(r):
			return "", 0, false
		}
	}
	return "", 0, false
}

// isNameRune reports whether r can be part of a word an @ is embedded in,
// as in an email address.
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-'
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
		if prev, _ := utf8.DecodeLastRuneInString(content[:m[0]]); m[0] > 0 && isNameRune(prev) {
			continue // an email address
		}
		spans = append(spans, MentionSpan{
			Start:    m[0],
			End:      m[1],
			Username: content[m[2]:m[3]],
		})
	}
	return spans
}
