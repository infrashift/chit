// A stand-in for HALO's file in the shared keto-cue schema directory. It
// follows the same rules as deploy/keto/schema/chit.cue: package keto, its
// own "halo/" prefix, and a reference to a Chit namespace. `make
// keto-validate` compiles it next to chit.cue.
package keto

#halo_invokers: ["chit/actor"]

namespaces: "halo/tool": relations: invoke: #halo_invokers
