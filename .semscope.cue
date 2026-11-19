// The semscope overlay of chit (make semscope). ADR-0002 in .okfw/decisions
// adapts the Power of 10 rules to Go, and ADR-0003 records why semscope
// checks them.
package overlay

// R3 deviation (ADR-0002): Go, pgx, chi, Bubble Tea and CUE allocate memory
// at run time. Hard limits cap the size of requests, pages and messages.
rules: "NASA-P10-03": enabled: false

// R7 with type information: each call, defer and go statement that drops an
// error is a finding.
rules: "NASA-P10-07": params: go_types: true

// The UAT seed tools are not part of a chit binary.
exclude: scripts: "UAT seed tools; not shipped in a chit binary"

// The docs site has no Go code. Its node_modules is large.
exclude: docs: "the Astro docs site; no Go code"

// Refactor omissions. Each phase of PLAN.md fixes one rule and removes its
// line. At the end of Phase 9 this list is empty.
rules: "NASA-P10-02": enabled: false // Phase 5
rules: "NASA-P10-04": enabled: false // Phase 8
rules: "NASA-P10-05": enabled: false // Phase 9
rules: "NASA-P10-06": enabled: false // Phase 7
rules: "NASA-P10-09": enabled: false // Phase 6
