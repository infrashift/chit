---
id: "01a1f537-9e0c-71ee-95a3-e988d46249b3"
title: "ADR-0005: lexis checks the documentation with a repository vocabulary"
type: "ADR"
tags:
  - "adr"
  - "docs"
  - "ste"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "make docs-lexis checks the Markdown documentation and the wiki against ASD-STE100 with lexis and vocabulary.cue, and each finding fails it."
---

# ADR-0005: lexis checks the documentation with a repository vocabulary

**Status:** Accepted
**Date:** 2026-11-19
**Source:** User request. semscope ADR-0015 makes the same decision.

## Context
The user requires ASD-STE100 Simplified Technical English for the documentation. lexis checks Markdown with its `ste-100` ruleset. On 2026-11-19, lexis gave 1,971 findings in the chit Markdown files. Some findings name a project term, for example "provisioning" or "identity". A verb in place of such a term changes what the text says.

## Decision
- `vocabulary.cue` holds the technical terms of chit. lexis loads it over the built-in ruleset: `--ruleset ste-100 --ruleset vocabulary.cue`.
- Add a term only when it names a concept of the project, a tool or a standard. Write other text again.
- `make docs-lexis` scans `README.md`, `CLAUDE.md`, `PLAN.md`, `PRD.md`, `PRD-SLASH-COMMANDS.md`, `SPECS-SLASH-COMMANDS.md`, `ROADMAP.md`, `tests/e2e/README.md`, the TUI Markdown files, `docs/src/content/docs` and `.okfw` with `--fail-on note`.
- The gate skips the history: `CHANGELOG.md`, the TUI `CHANGELOG.md`, `changelog.mdx`, `clients/chit-tui/docs/history`, the daily notes, `index.md` and `log.md`.
- The gate does not scan Go comments.
- During the refactor, `DOCS_LEXIS_PENDING` holds the files that still have findings. Phase 10 moves them to `DOCS_LEXIS_PATHS`, so each commit passes the gate.
- `docs-lexis` runs lexis only when `command -v` finds it. Otherwise it prints a note and passes. The variable `LEXIS` names the command.

## Consequences
- On 2026-11-19, the gate checks 69 files with zero findings. The vocabulary holds 26 terms, each with a reason. Three agents wrote the other text again.
- A new project term needs new text, or a vocabulary entry with a reason.
- A changelog entry keeps its original words.

## Alternatives considered
- **Fail on errors only.** Rejected: lexis reports noun forms, -ing forms and the passive voice as warnings.
- **Scan Go comments.** Rejected by the user for now.
