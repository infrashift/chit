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

	// Security and identity (the Ory stack, ADR-0006).
	authentication: true // Kratos sessions and Hydra tokens
	authorization:  true // permission checks in chitd and Keto
	audience:       true // the OAuth2 audience of a Hydra token
	subscription:   true // a Claude subscription, and a pub/sub subscription

	// Server features and their names in the API and the docs.
	migration:    true // a database schema migration in migrations/
	pagination:   true // the page and per_page query parameters
	notification: true // a PostgreSQL NOTIFY, and an @mention notification
	validation:   true // request validation against the OpenAPI document
	pinning:      true // the message pinning feature
	rendering:    true // Markdown rendering in a client
	attachment:   true // an image file on a post (P-CHIT-5, file_ids, attachment_ids)
	provenance:   true // props.provenance: what produced an agent's message

	// The TUI and the test process.
	appearance: true // the appearance setting of chit-tui: dark or light
	navigation: true // keyboard navigation in chit-tui
	testing:    true // user acceptance testing and manual acceptance testing
}
