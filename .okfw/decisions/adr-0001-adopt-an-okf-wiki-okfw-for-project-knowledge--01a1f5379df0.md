---
id: "01a1f537-9df0-7aa7-b006-a0254ea3a5ff"
title: "ADR-0001: Adopt an OKF wiki (okfw) for project knowledge"
type: "ADR"
tags:
  - "adr"
  - "process"
  - "okf"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "An okfw OKF wiki in .okfw/ holds chit project knowledge: architecture, specs, ADRs, issues and daily notes."
---

# ADR-0001: Adopt an OKF wiki (okfw) for project knowledge

**Status:** Accepted
**Date:** 2026-11-19
**Source:** User request. zmake ADR-0001 and semscope ADR-0001 make the same decision.

## Context
chit has many decisions that the code does not explain: the Ory stack, the thin pub/sub envelopes, the separate TUI module and the on-behalf-of requests. The reasons are only in commit messages and in `CHANGELOG.md`. The infrashift repos zmake, semscope and lexis use okfw.

## Decision
- An OKF wiki lives at `.okfw/` in the repository root. The `okfw` CLI manages it. The repository contains it.
- `okfw install --platform claude` adds the Claude Code skill and the managed block of `CLAUDE.md`. `.mcp.json` registers `okfw mcp`.
- The wiki uses the zmake layout: `architecture/`, `specs/`, `decisions/`, `issues/` and `daily/`.
- `PLAN.md` is the phase checklist. Each phase has a mirror issue in `issues/`.
- Rules: [Wiki conventions](/wiki-conventions--01a1f5379dca.md). Loop: [Development workflow](/development-workflow--01a1f5379dcf.md).

## Consequences
- `okfw search` finds decisions and open work. Links make a graph of them.
- `PLAN.md` and the issues can drift apart. The workflow tells you to update both.
- `.gitignore` ignores only the local settings of `.claude/`, so the okfw skill is in the repository.

## Alternatives considered
- **Plain `docs/` markdown.** Rejected: no frontmatter, no typed search and no generated index.
- **`ROADMAP.md` only.** Rejected: it lists features, not the reasons for decisions.
