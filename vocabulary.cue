// The vocabulary of the chit repository: technical terms of this project
// that the lexis rules STE-100-06 and STE-100-07 do not report. The
// docs-lexis gate loads this file over the built-in ste-100 ruleset of lexis
// (ADR-0005 in .okfw/decisions):
//
//	lexis scan --ruleset ste-100 --ruleset vocabulary.cue ...
//
// Add a term only when it names a concept of the project, a tool or a
// standard. Rewrite other prose.
vocabulary: {
	// Project process: PLAN.md, ADRs and issues.
	acceptance:  true // acceptance criteria of an issue
	decision:    true // architecture decision record
	development: true // the development workflow
	resolution:  true // the Resolution section of an issue
	tracking:    true // the Tracking line of a PLAN.md phase

	// The Power of 10 rules and the nasa-p10 ruleset (ADR-0002).
	allocation:  true // Power of 10 rule R3
	indirection: true // Power of 10 rule R9
	deviation:   true // a Power of 10 rule that chit does not obey
	suppression: true // a semscope allow comment
	omission:    true // a rule that .semscope.cue omits during the refactor
	adaptation:  true // a Power of 10 rule as ADR-0002 changes it for Go

	// chit concepts.
	provisioning: true // the first-request creation of a user from a Kratos identity
	conversation: true // a HALO Conversation: a private channel for one person
	integration:  true // the integration tests and the integration build tag
	marketing:    true // the marketing words of the style rules
}
