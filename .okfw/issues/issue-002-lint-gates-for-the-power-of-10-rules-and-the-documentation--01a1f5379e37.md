---
x-okfw-id: "01a1f537-9e37-7161-9e3c-18eb14314576"
title: "ISSUE-002: Lint gates for the Power of 10 rules and the documentation"
type: "Issue"
x-okfw-kind: "issue"
tags:
  - "issue"
  - "phase-2"
  - "lint"
  - "tooling"
  - "closed"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:14:08-06:00"
description: "make check runs golangci-lint, semscope, the tests, lexis and the wiki checks, with the unfixed rules omitted."
---

# ISSUE-002: Lint gates for the Power of 10 rules and the documentation

**Status/Phase:** Closed · Phase 2

## Description
make check runs golangci-lint, semscope, the tests, lexis and the wiki checks, with the unfixed rules omitted.

## Acceptance Criteria
- [x] `.semscope.cue` turns off NASA-P10-03, excludes `scripts`, and omits each rule that still has findings.
- [x] `vocabulary.cue` exists.
- [x] `make semscope`, `make docs-lexis`, `make wiki` and `make check` exist.
- [x] `make lint-tui` uses `GOLANGCI_LINT`.
- [x] golangci-lint reports zero issues in both modules (R10).
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 2 ([ADR-0003](/decisions/adr-0003-semscope-checks-the-power-of-10-rules--01a1f5379dfe.md), [ADR-0005](/decisions/adr-0005-lexis-checks-the-documentation-with-a-repository-vocabulary--01a1f5379e0c.md)).

- `.semscope.cue` turns off NASA-P10-03, sets `go_types` for NASA-P10-07, and excludes `scripts` and `docs`. It omits seven rules until their phases.
- `make semscope`, `make docs-lexis`, `make wiki` and `make check` exist. `docs-lexis` scans `CLAUDE.md`, `PLAN.md` and `.okfw`. `DOCS_LEXIS_PENDING` holds the other files until Phase 10.
- `make lint-tui` passes `GOLANGCI_LINT` to the TUI Makefile.
- R10: the test of the webhook dispatcher checks the error of `Start`. `internal/app/command_membership.go` uses `%#q`. golangci-lint reports zero issues in both modules.
- `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
