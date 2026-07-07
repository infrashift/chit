package palette

import "strings"

// fuzzyScore reports whether query is a case-insensitive subsequence of
// candidate and how good the match is. Higher scores are better matches:
// consecutive runs and matches at the start of the candidate score higher.
// An empty query matches everything with score 0.
func fuzzyScore(query, candidate string) (int, bool) {
	q := []rune(strings.ToLower(query))
	c := []rune(strings.ToLower(candidate))
	if len(q) == 0 {
		return 0, true
	}
	if len(q) > len(c) {
		return 0, false
	}

	score := 0
	qi := 0
	prevMatch := -2
	for ci := 0; ci < len(c) && qi < len(q); ci++ {
		if c[ci] != q[qi] {
			continue
		}
		score++
		if ci == prevMatch+1 {
			score += 2 // consecutive-run bonus
		}
		if ci == qi {
			score++ // prefix-alignment bonus
		}
		prevMatch = ci
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	return score, true
}
