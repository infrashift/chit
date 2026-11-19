package model

import (
	"regexp"
	"strings"

	"github.com/infrashift/chit/internal/assert"
)

// mentionRe matches @username patterns at word boundaries. It uses the same
// character set as validUsernameRe. chitd resolves mentions with it and the
// chit-claude bridge decides whether it was named with it, so the two cannot
// disagree about what counts as a mention.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
var mentionRe = regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])@([a-z0-9][a-z0-9._-]{0,62}[a-z0-9])`)

// ParseMentions extracts deduplicated, lowercased usernames from content.
// @all and @channel come back like any other name; callers decide what they
// mean.
func ParseMentions(content string) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var result []string
	for _, m := range matches {
		assert.That(len(m) == 2, "ParseMentions: match without its one group")
		username := strings.ToLower(m[1])
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		result = append(result, username)
	}
	assert.That(len(result) > 0 && len(result) <= len(matches), "ParseMentions: result size out of range")
	assert.That(len(result) == len(seen), "ParseMentions: result has a duplicate")
	return result
}
