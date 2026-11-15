// Chit's part of the keto-cue permission schema.
//
// One keto-cue instance serves Chit and HALO. keto-cue compiles every *.cue
// file of its schema directory (namespaces.location) as ONE CUE package, so
// each service ships one file and every file follows these rules:
//
//   - The package clause is `package keto`, the name of keto-cue's base
//     schema. Two different package names in one directory are an error.
//   - A service declares namespaces only under its own prefix: Chit uses
//     "chit/...". CUE unifies a namespace that two files declare instead of
//     refusing it, so a shared name would silently merge two services' rules.
//   - Definitions (#name) and hidden fields (_name) are package-wide too:
//     prefix them with the service name (#chit_...).
//   - A file may refer to another service's namespace as a subject type,
//     for example ["chit/actor"].
//
// `make keto-validate` compiles this file alone and next to a sample HALO
// file (internal/keto/testdata/halo.cue). The chit-keto-schema ConfigMap in
// deploy/chit.kube.yml and deploy/chit-uat.kube.yml carries a copy, and
// TestKetoSchemaInSync keeps them the same.
//
// The schema declares only what Chit writes and checks. Keto runs in its
// default, non-strict mode, so a check follows every subject set, exactly as
// the namespace list of Keto v0.12 did.
package keto

namespaces: {
	// The subject type of the subject IDs that Chit writes: a user ID in
	// chit/channel, and "Actor:<user ID>" in chit/command. Keto does not
	// type-check a subject ID; this namespace only names it. No tuple lives
	// here.
	"chit/actor": {}

	// A mirror of channel_members, written best-effort. Chit never checks it
	// to grant access: <channel ID>#member@<user ID>.
	"chit/channel": relations: member: ["chit/actor"]

	// Slash-command authorization. chit-reconcile owns this namespace.
	//   Role:<name>#member@Actor:<user ID>
	//   Command:<slug>#execute@chit/command:Role:<name>#member
	"chit/command": relations: {
		member: ["chit/actor"]
		execute: ["chit/command#member"]
	}
}
