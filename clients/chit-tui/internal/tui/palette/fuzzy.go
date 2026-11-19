package palette

import (
	"strings"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

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
		assert.That(ci > prevMatch, "fuzzyScore: matches out of order")
		assert.That(qi <= ci, "fuzzyScore: more query runes matched than candidate runes read")
		prevMatch = ci
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	// Each matched rune scores 1, plus at most 2 for a run and 1 for alignment.
	assert.That(score >= len(q), "fuzzyScore: a match scored less than one per rune")
	assert.That(score <= 4*len(q), "fuzzyScore: a match scored more than four per rune")
	return score, true
}
