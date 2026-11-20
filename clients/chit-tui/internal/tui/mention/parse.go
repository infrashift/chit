package mention

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
//
// semscope: allow(NASA-P10-06) compiled once, never changed
var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_](?:[a-zA-Z0-9_.\-]*[a-zA-Z0-9_])?)`)

// ExtractMentionPrefix scans backward from cursorCol in text to find an @mention prefix.
// Returns the prefix after @, the column of the @, and whether a mention trigger is active.
// Columns count characters, not bytes, as the input's cursor does. An @
// inside a word is an email address and does not trigger.
func ExtractMentionPrefix(text string, cursorCol int) (prefix string, startCol int, active bool) {
	assert.That(cursorCol >= 0, "ExtractMentionPrefix: negative cursor column")
	runes := []rune(text)
	cursorCol = min(cursorCol, len(runes))
	for i := cursorCol - 1; i >= 0; i-- {
		switch r := runes[i]; {
		case r == '@':
			if i > 0 && isNameRune(runes[i-1]) {
				return "", 0, false
			}
			prefix := string(runes[i+1 : cursorCol])
			assert.That(!strings.ContainsFunc(prefix, unicode.IsSpace), "ExtractMentionPrefix: prefix spans a space")
			assert.That(!strings.ContainsRune(prefix, '@'), "ExtractMentionPrefix: prefix holds another @")
			return prefix, i, true
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
	out := append(specials, normals...)
	assert.That(len(out) <= len(entries), "FilterEntries: more matches than entries")
	assert.That(lower != "" || len(out) == len(entries), "FilterEntries: empty prefix dropped entries")
	for i := 1; i < len(out); i++ {
		assert.That(out[i-1].Special || !out[i].Special, "FilterEntries: special mention after a user")
	}
	return out
}

// FindMentions finds all @mention spans in content.
func FindMentions(content string) []MentionSpan {
	matches := mentionRe.FindAllStringSubmatchIndex(content, -1)
	var spans []MentionSpan
	for _, m := range matches {
		assert.That(content[m[0]] == '@', "FindMentions: match does not start at an @")
		assert.That(m[2] == m[0]+1, "FindMentions: name does not follow the @")
		assert.That(m[2] < m[3] && m[3] == m[1], "FindMentions: empty or trailing-text name")
		assert.That(m[1] <= len(content), "FindMentions: match past the content")
		// size is 0 only when the mention starts the content.
		if prev, size := utf8.DecodeLastRuneInString(content[:m[0]]); size > 0 && isNameRune(prev) {
			continue // an email address
		}
		assert.That(len(spans) == 0 || spans[len(spans)-1].End <= m[0], "FindMentions: spans overlap or out of order")
		spans = append(spans, MentionSpan{
			Start:    m[0],
			End:      m[1],
			Username: content[m[2]:m[3]],
		})
		name := spans[len(spans)-1].Username
		assert.That(!strings.ContainsRune(name, '@'), "FindMentions: name holds an @")
		assert.That(!strings.HasSuffix(name, ".") && !strings.HasSuffix(name, "-"), "FindMentions: name ends in punctuation")
	}
	assert.That(len(spans) <= len(matches), "FindMentions: more spans than matches")
	return spans
}
