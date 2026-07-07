package auth

// Actor→role bindings. Each entry assigns a Chit user (by its users.id UUID)
// to one or more roles defined under auth/roles/. `chit-reconcile` pushes
// these as Keto relation tuples (Role:<name> member Actor:<id>), which is what
// authorizes slash-command execution for that user.
//
// Humans, AI agents, and bots are bound the same way — an agent is just a
// user row with actor_type=agent whose ID appears here with the "agent" role.
//
// Example (uncomment and substitute the real user ID, e.g. the chit-agent
// user created by scripts/seed-uat):
//
// actors: "chit-agent": {
// 	id: "019421a0-0000-7000-8000-00000000abcd"
// 	roles: ["agent"]
// }
