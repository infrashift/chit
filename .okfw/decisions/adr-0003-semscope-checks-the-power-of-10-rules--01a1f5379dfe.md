---
id: "01a1f537-9dfe-71bc-9349-e0c32461764b"
title: "ADR-0003: semscope checks the Power of 10 rules"
type: "ADR"
tags:
  - "adr"
  - "lint"
  - "power-of-10"
  - "tooling"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "make semscope runs semscope with the nasa-p10 ruleset and the .semscope.cue overlay on both Go modules when semscope is installed."
---

# ADR-0003: semscope checks the Power of 10 rules

**Status:** Accepted
**Date:** 2026-11-19
**Source:** User request. zmake ADR-0026 makes the same decision.

## Context
golangci-lint does not check recursion, loop bounds or assertion density. semscope checks the ten rules of [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) with a Tree-sitter parser. The semscope repository has no remote, so CI cannot install it.

## Decision
- `make semscope` runs `semscope scan -r nasa-p10 -r .semscope.cue --fail-level warning . clients/chit-tui` when `semscope` is on the `PATH`. Otherwise it prints a note and passes.
- The variable `SEMSCOPE` names the command.
- `.semscope.cue` turns off NASA-P10-03 (the R3 deviation) and sets `NASA-P10-07.go_types: true`. It excludes `scripts`, `docs`, `tests/e2e/hookstub` and the three packages of test doubles. The file gives the reason for each path.
- During the refactor, `.semscope.cue` also omits each rule that still has findings. Each phase removes one omission. At the end, the list is empty.
- `make check` runs `semscope`.

## Consequences
- A local run checks ten rules. CI checks the golangci-lint linters of [ADR-0004](/decisions/adr-0004-golangci-lint-enforces-the-power-of-10-linters--01a1f5379e05.md) only.
- semscope is a development tool, not a dependency of chit.

## Alternatives considered
- **golangci-lint only.** Rejected: it does not see recursion, `for {}` or the assertion count.
