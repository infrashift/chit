# PLAN.md — chit

This file is the phase checklist. Each phase has a mirror issue in `.okfw/issues/`. The *Development workflow* guide in `.okfw/` gives the loop for each phase.

## Phase 1: OKF wiki and MCP server
Tracking: [ISSUE-001](.okfw/issues/issue-001-okf-wiki-and-mcp-server--01a1f5379e2f.md)

- [x] Run `okfw init` and `okfw install --platform claude`.
- [x] Add `.mcp.json` with `okfw mcp`.
- [x] Write the guides, the architecture overview, the specs and ADR-0001 to ADR-0009.
- [x] Write `CLAUDE.md`, `PLAN.md` and `vocabulary.cue`.

## Phase 2: Lint gates
Tracking: [ISSUE-002](.okfw/issues/issue-002-lint-gates-for-the-power-of-10-rules-and-the-documentation--01a1f5379e37.md)

- [x] Add `.semscope.cue` with the R3 deviation and the omissions.
- [x] Add the `semscope`, `docs-lexis`, `wiki` and `check` targets.
- [x] Fix the golangci-lint issues of the baseline (R10).

## Phase 3: R7, check each return value
Tracking: [ISSUE-003](.okfw/issues/issue-003-r7-check-each-return-value--01a1f5379e3e.md)

- [x] Fix each discarded result and each dropped error.
- [x] Remove the NASA-P10-07 omission. Turn on `errcheck` `check-blank` and `errorlint`.

## Phase 4: R1, no recursion
Tracking: [ISSUE-004](.okfw/issues/issue-004-r1-no-recursion--01a1f5379e46.md)

- [ ] Remove the two recursion cycles in chit-tui.
- [ ] Remove the NASA-P10-01 omission. Turn on `gocognit` and `nestif`.

## Phase 5: R2, loops with a bound
Tracking: [ISSUE-005](.okfw/issues/issue-005-r2-loops-with-a-bound--01a1f5379e4d.md)

- [ ] Replace each `for {}` with an exit condition or a maximum count.
- [ ] Remove the NASA-P10-02 omission.

## Phase 6: R9, no function types
Tracking: [ISSUE-006](.okfw/issues/issue-006-r9-no-function-types-in-chit-code--01a1f5379e55.md)

- [ ] Replace the fields of function type and the dispatch tables.
- [ ] Suppress the function types that a framework requires.
- [ ] Remove the NASA-P10-09 omission.

## Phase 7: R6, no mutable package variables
Tracking: [ISSUE-007](.okfw/issues/issue-007-r6-no-mutable-package-variables--01a1f5379e5d.md)

- [ ] Move or remove the package variables.
- [ ] Remove the NASA-P10-06 omission. Turn on `gochecknoglobals`.

## Phase 8: R4, functions of 60 lines or fewer
Tracking: [ISSUE-008](.okfw/issues/issue-008-r4-functions-of-60-lines-or-fewer--01a1f5379e65.md)

- [ ] Split each long function.
- [ ] Remove the NASA-P10-04 omission. Turn on `funlen`.

## Phase 9: R5, assertion density
Tracking: [ISSUE-009](.okfw/issues/issue-009-r5-assertion-density--01a1f5379e6c.md)

- [ ] Add `internal/assert` to each module.
- [ ] Add assertions to an average of two for each function.
- [ ] Add recover guards to the goroutines.
- [ ] Remove the NASA-P10-05 omission. Turn on `forbidigo`.

## Phase 10: ASD-STE100 documentation
Tracking: [ISSUE-010](.okfw/issues/issue-010-asd-ste100-documentation--01a1f5379e74.md)

- [ ] Write the Markdown documentation again until `make docs-lexis` reports zero findings.
