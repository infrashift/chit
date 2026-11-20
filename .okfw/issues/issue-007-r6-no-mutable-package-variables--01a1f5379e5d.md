---
id: "01a1f537-9e5d-74b7-bc9b-7c054a383fef"
title: "ISSUE-007: R6: no mutable package variables"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-7"
  - "power-of-10"
  - "r6"
  - "closed"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:54:30-06:00"
description: "Move or remove package variables, and suppress only the kinds that ADR-0002 allows."
---

# ISSUE-007: R6: no mutable package variables

**Status/Phase:** Closed · Phase 7

## Description
Move or remove package variables, and suppress only the kinds that ADR-0002 allows.

## Acceptance Criteria
- [x] Each package variable is gone or has a suppression with a reason.
- [x] `gochecknoglobals` is on.
- [x] NASA-P10-06 is not omitted.
- [x] `make check` passes.

## Resolution
Done on 2026-11-19 in Phase 7.

- Lookup tables and fixed lists are now functions or `switch` statements. Examples: the on-behalf-of routes, the help bindings, the theme keys and the terminal color names.
- `AuthExtract` builds the on-behalf-of route table once, when it builds the middleware. The refusal text does not change.
- Mutable state moved into structs. The clipboard writer is a field of the TUI `Model`. The cache of the terminal query is a field of `terminalBackground`.
- These package variables stay, each with a suppression and a reason: 3 `//go:embed` variables, 11 sentinel errors and 12 compiled regular expressions.
- `config.ThemeColorKeys`, `ThemeDerivedKeys` and `ThemeMeta`, and `viewport.ReplyKey`, `PrevPostKey` and `NextPostKey`, are now functions.
- Bug fix: `SaveThemeSetting` read a `$` in a theme name as a group reference. It now uses `ReplaceAllLiteralString`. A new test covers it.
- semscope NASA-P10-06 reports zero findings, and `.semscope.cue` does not omit it. `gochecknoglobals` is on in both modules.
- `make check` and `make test-integration` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
