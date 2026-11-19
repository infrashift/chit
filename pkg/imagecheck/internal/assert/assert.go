// Package assert holds the run-time assertions of NASA Power of 10 rule 5
// for imagecheck. It is a copy of Chit's internal/assert, so imagecheck
// keeps no Chit dependencies (HALO reuses it).
//
// An assertion checks a programmer error: a broken precondition, invariant
// or postcondition. Code that receives bad input returns an error instead.
// Assertions have no side effects.
package assert

// That panics with the message msg when cond is false.
func That(cond bool, msg string) {
	if !cond {
		panic("imagecheck: assertion failed: " + msg)
	}
}
