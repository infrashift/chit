---
id: "01a1f537-9df7-7518-9bc8-d0497bca465d"
title: "ADR-0002: Adapt the NASA Power of 10 rules to chit"
type: "ADR"
tags:
  - "adr"
  - "safety"
  - "power-of-10"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "chit obeys the NASA/JPL Power of 10 rules as adapted to Go, with a deviation for R3 and framework exceptions for R6 and R9."
---

# ADR-0002: Adapt the NASA Power of 10 rules to chit

**Status:** Accepted
**Date:** 2026-11-19
**Source:** User request. Gerard J. Holzmann, "The Power of 10" (2006). semscope ADR-0003 adapts the rules to Go.

## Context
The user requires the Power of 10 rules. The rules are for C. chit is a Go server and a Go terminal client. Go has garbage collection, closures, interfaces and no `assert` statement. chitd and chit-tui run until shutdown. net/http, chi and Bubble Tea require some function types. On 2026-11-19, semscope found 645 findings in chit: 226 errors and 419 R3 warnings.

## Decision
| Rule | chit adaptation | Check |
|---|---|---|
| R1 No recursion | No `goto` and no recursion. A walk uses a loop or an explicit stack. A Bubble Tea handler that needs `Update` again returns a `tea.Cmd`. | semscope, `gocognit`, `nestif` |
| R2 Loop bounds | No `for {}`. A loop that runs until shutdown states its exit condition: `for ctx.Err() == nil`. A retry loop has a maximum count. | semscope |
| R3 No allocation after initialization | **Deviation.** Go, pgx, chi, Bubble Tea and CUE allocate at run time. Hard limits cap the size of requests, pages and messages ([chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)). | `.semscope.cue` turns it off |
| R4 Short functions | 60 lines or fewer. | semscope, `funlen` |
| R5 Assertions | An average of two or more `assert.That` calls for each function. Each module has `internal/assert`. Assertions check programmer errors only. Bad input returns an error. | semscope, `forbidigo` |
| R6 Smallest scope | No mutable package variables. These stay, each with a suppression: `//go:embed` variables, sentinel errors, compiled regular expressions that do not change, and compile-time interface checks. | semscope, `gochecknoglobals` |
| R7 Check return values | Check each error. No `_ = f()`. No blank error result. A `defer` close joins its error or logs it. | semscope with `go_types`, `errcheck` |
| R8 Limited preprocessor | Build tags for the `integration` and `e2e` test sets only. No `//go:linkname`. | semscope, review |
| R9 Restricted pointers | No function types, fields of function type or dispatch tables in chit code. Use an interface or a `switch`. A signature that net/http, chi or Bubble Tea requires stays, with a suppression. | semscope |
| R10 Zero warnings | `go vet` and golangci-lint report zero issues in both modules. No `unsafe`. | `make lint lint-tui` |

**Assertions:** `assert.That(cond, msg)` panics when `cond` is false. The HTTP recover middleware changes a panic in a handler into a 500 response. Each worker, pump and listener goroutine has a recover guard that logs the panic.

**Suppression:** `// semscope: allow(NASA-P10-NN) <reason>` on the line or the line above. Each suppression has a reason.

**Scope:** semscope scans the server module and `clients/chit-tui`. It skips the test files and `scripts/`, the UAT seed tools. It also skips two packages of test doubles: `internal/store/storetest` and `clients/chit-tui/internal/testutil`. Only test files import them. golangci-lint checks all of these files.

## Consequences
- The code is longer: explicit bounds, small functions and assertions.
- A failed assertion in a request handler gives a 500 response, not a crash.
- The refactor uses one phase for each rule. Each phase removes one omitted rule from `.semscope.cue` ([ADR-0003](/decisions/adr-0003-semscope-checks-the-power-of-10-rules--01a1f5379dfe.md)).

## Alternatives considered
- **Literal R3.** Rejected: Go cannot obey it.
- **No assertions (R5 deviation).** Rejected by the user.
- **Remove all function types.** Rejected: net/http middleware and Bubble Tea commands need them.
