---
id: "01a1f537-9e37-7161-9e3c-18eb14314576"
title: "ISSUE-002: Lint gates for the Power of 10 rules and the documentation"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "open"
  - "phase-2"
  - "lint"
  - "tooling"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "make check runs golangci-lint, semscope, the tests, lexis and the wiki checks, with the unfixed rules omitted."
---

# ISSUE-002: Lint gates for the Power of 10 rules and the documentation

**Status/Phase:** Open · Phase 2

## Description
make check runs golangci-lint, semscope, the tests, lexis and the wiki checks, with the unfixed rules omitted.

## Acceptance Criteria
- [ ] `.semscope.cue` turns off NASA-P10-03, excludes `scripts`, and omits each rule that still has findings.
- [ ] `vocabulary.cue` exists.
- [ ] `make semscope`, `make docs-lexis`, `make wiki` and `make check` exist.
- [ ] `make lint-tui` uses `GOLANGCI_LINT`.
- [ ] golangci-lint reports zero issues in both modules (R10).
- [ ] `make check` passes.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
