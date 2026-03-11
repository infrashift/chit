package tagpicker

import (
	"regexp"
	"strings"
)

var hashtagRe = regexp.MustCompile(`(?:^|\s)#([a-zA-Z0-9_-]+)`)

// ExtractHashtags returns tag names found in content.
func ExtractHashtags(content string) []string {
	matches := hashtagRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var tags []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if !seen[name] {
			seen[name] = true
			tags = append(tags, name)
		}
	}
	return tags
}

// StripHashtags removes #tag tokens from content, returning cleaned content
// and the list of extracted tag names.
func StripHashtags(content string) (string, []string) {
	tags := ExtractHashtags(content)
	if len(tags) == 0 {
		return content, nil
	}
	cleaned := hashtagRe.ReplaceAllString(content, "")
	cleaned = strings.TrimSpace(cleaned)
	// Collapse multiple spaces
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}
	return cleaned, tags
}
