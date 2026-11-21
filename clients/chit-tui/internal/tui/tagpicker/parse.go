package tagpicker

import (
	"regexp"
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// hashtagRe matches a #tag at the start of a line or after a space or tab.
// A tag needs a letter, so "#123" stays an issue or PR reference. Letters
// and digits are Unicode, as the server permits. The server also permits
// '.' and spaces in a name, but a #tag stops at them: "see #docs." does not
// take the period.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
var hashtagRe = regexp.MustCompile(`(^|[ \t])#([\p{L}\p{N}_-]*\p{L}[\p{L}\p{N}_-]*)`)

// ExtractHashtags returns tag names found in content, lowercased and
// deduplicated. Code, fenced or inline, is quoted verbatim and never holds
// tags: "#include" in a C snippet is not one.
func ExtractHashtags(content string) []string {
	_, tags := scanHashtags(content, false)
	// Every tag is matched at its own '#'.
	assert.That(len(tags) <= strings.Count(content, "#"), "ExtractHashtags: more tags than '#' marks")
	for _, t := range tags {
		assert.That(t != "" && t == strings.ToLower(t), "ExtractHashtags: tag empty or not lowercased")
	}
	return tags
}

// StripHashtags removes #tag tokens from content, returning cleaned content
// and the list of extracted tag names. Only the tags go, each with the one
// space that separated it; indentation, runs of spaces and line breaks are
// left as written, and a line that held nothing but tags is dropped.
func StripHashtags(content string) (string, []string) {
	cleaned, tags := scanHashtags(content, true)
	assert.That(len(cleaned) <= len(content), "StripHashtags: stripping made the content longer")
	assert.That(len(tags) <= strings.Count(content, "#"), "StripHashtags: more tags than '#' marks")
	return cleaned, tags
}

func scanHashtags(content string, strip bool) (string, []string) {
	sc := &hashtagScan{strip: strip, seen: make(map[string]bool)}
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

		cleaned, removed := sc.scanLine(line)
		switch {
		case !removed:
			out = append(out, line)
		case cleaned != "":
			out = append(out, cleaned)
		}
	}

	assert.That(len(out) <= len(lines), "scanHashtags: more output lines than input lines")
	assert.That(len(sc.seen) == len(sc.tags), "scanHashtags: seen set and tag list out of step")
	if !strip {
		return content, sc.tags
	}
	cleaned := strings.Trim(strings.Join(out, "\n"), "\n")
	assert.That(strings.Count(cleaned, "\n") <= strings.Count(content, "\n"), "scanHashtags: stripping added lines")
	return cleaned, sc.tags
}

// hashtagScan collects the tags of one piece of content, in order of first
// appearance, and with strip set removes them from it.
type hashtagScan struct {
	strip bool
	seen  map[string]bool
	tags  []string
}

// scanLine scans one line outside a code fence. It returns the line with its
// tags removed and whether any were; a line that lost none is left to the
// caller as written.
func (sc *hashtagScan) scanLine(line string) (string, bool) {
	// Backticks alternate text and inline code: even parts are text.
	// An unmatched final backtick opens no span, so the part after it
	// is text as well.
	assert.That(sc.seen != nil, "hashtagScan.scanLine: nil seen set")
	parts := strings.Split(line, "`")
	assert.That(len(parts) == strings.Count(line, "`")+1, "scanLine: split lost a part")
	removed := false
	for i := range parts {
		unmatched := i == len(parts)-1 && len(parts)%2 == 0
		if i%2 == 1 && !unmatched {
			continue
		}
		var r bool
		parts[i], r = sc.scanText(parts[i], i > 0)
		removed = removed || r
	}
	assert.That(sc.strip || !removed, "scanLine: removed a tag while only extracting")
	if !removed {
		return line, false
	}
	cleaned := strings.TrimRight(strings.Join(parts, "`"), " \t")
	assert.That(len(cleaned) < len(line), "hashtagScan.scanLine: removing tags did not shorten the line")
	return cleaned, true
}

// scanText scans one stretch of text between inline code spans; afterCode
// says whether inline code precedes it on the line. It returns the text,
// without its tags when stripping, and whether any were removed.
func (sc *hashtagScan) scanText(text string, afterCode bool) (string, bool) {
	assert.That(sc.seen != nil, "hashtagScan.scanText: nil seen set")
	var kept strings.Builder
	last, skipSpace, removed := 0, false, false
	for _, mt := range hashtagRe.FindAllStringSubmatchIndex(text, -1) {
		start, end, lead := mt[0], mt[1], mt[3]-mt[2]
		assert.That(start >= last && end <= len(text), "scanText: match outside the unscanned text")
		assert.That(mt[4] < mt[5], "scanText: empty tag name matched")
		// "^" is the start of the segment; only at the start of the
		// line does it mean a tag, not text right after inline code.
		if lead == 0 && afterCode {
			continue
		}
		sc.add(strings.ToLower(text[mt[4]:mt[5]]))
		if !sc.strip {
			continue
		}
		between := text[last:start]
		if skipSpace {
			between = trimOneSpace(between)
		}
		kept.WriteString(between)
		last = end
		// A tag that opened the line takes the space after it.
		skipSpace = lead == 0
		removed = true
	}
	if !sc.strip {
		return text, false
	}
	assert.That(last <= len(text), "scanText: scan position past the text")
	rest := text[last:]
	if skipSpace {
		rest = trimOneSpace(rest)
	}
	kept.WriteString(rest)
	assert.That(kept.Len() <= len(text), "scanText: kept more than the text")
	return kept.String(), removed
}

// add records a tag name the first time it is seen.
func (sc *hashtagScan) add(name string) {
	assert.That(name != "", "hashtagScan.add: empty tag name")
	assert.That(sc.seen != nil, "hashtagScan.add: nil seen set")
	if !sc.seen[name] {
		sc.seen[name] = true
		sc.tags = append(sc.tags, name)
	}
	assert.That(len(sc.seen) == len(sc.tags), "hashtagScan.add: seen set and tag list out of step")
}

func trimOneSpace(s string) string {
	if s != "" && (s[0] == ' ' || s[0] == '\t') {
		return s[1:]
	}
	return s
}
