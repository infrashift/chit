// Chit's keto-cue permission schema. keto-cue reads it from
// namespaces.location in keto.yml; `make keto-validate` compiles it the way
// the server does. The chit-keto-config ConfigMap in deploy/chit.kube.yml and
// deploy/chit-uat.kube.yml carries a copy, and TestKetoSchemaInSync keeps the
// three the same.
//
// The schema declares only what Chit writes and checks. Keto runs in its
// default, non-strict mode, so a check follows every subject set, exactly as
// the namespace list of Keto v0.12 did.
package chit

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
