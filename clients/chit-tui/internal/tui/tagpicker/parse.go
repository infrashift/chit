package tagpicker

import (
	"regexp"
	"strings"
)

// hashtagRe matches a #tag at the start of a line or after a space or tab.
// A tag needs a letter, so "#123" stays an issue or PR reference.
//
// semscope: allow(NASA-P10-06) compiled once, never changed
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

	if !strip {
		return content, sc.tags
	}
	return strings.Trim(strings.Join(out, "\n"), "\n"), sc.tags
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
	parts := strings.Split(line, "`")
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
	if !removed {
		return line, false
	}
	return strings.TrimRight(strings.Join(parts, "`"), " \t"), true
}

// scanText scans one stretch of text between inline code spans; afterCode
// says whether inline code precedes it on the line. It returns the text,
// without its tags when stripping, and whether any were removed.
func (sc *hashtagScan) scanText(text string, afterCode bool) (string, bool) {
	var kept strings.Builder
	last, skipSpace, removed := 0, false, false
	for _, mt := range hashtagRe.FindAllStringSubmatchIndex(text, -1) {
		start, end, lead := mt[0], mt[1], mt[3]-mt[2]
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
	rest := text[last:]
	if skipSpace {
		rest = trimOneSpace(rest)
	}
	kept.WriteString(rest)
	return kept.String(), removed
}

// add records a tag name the first time it is seen.
func (sc *hashtagScan) add(name string) {
	if !sc.seen[name] {
		sc.seen[name] = true
		sc.tags = append(sc.tags, name)
	}
}

func trimOneSpace(s string) string {
	if s != "" && (s[0] == ' ' || s[0] == '\t') {
		return s[1:]
	}
	return s
}
