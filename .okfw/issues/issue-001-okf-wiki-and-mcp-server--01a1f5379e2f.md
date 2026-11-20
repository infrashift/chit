---
id: "01a1f537-9e2f-7796-94c2-41a739c70a9b"
title: "ISSUE-001: OKF wiki and MCP server"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-1"
  - "wiki"
  - "okf"
  - "closed"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:11:37-06:00"
description: "chit has an okfw wiki with the zmake layout and the okfw MCP server in .mcp.json."
---

# ISSUE-001: OKF wiki and MCP server

**Status/Phase:** Closed · Phase 1

## Description
chit has an okfw wiki with the zmake layout and the okfw MCP server in .mcp.json.

## Acceptance Criteria
- [x] You ran `okfw init` and `okfw install --platform claude`.
- [x] `.mcp.json` registers `okfw mcp`.
- [x] The wiki has the two guides, the architecture overview, four specs and ADR-0001 to ADR-0009.
- [x] `CLAUDE.md` and `PLAN.md` exist.
- [x] `okfw validate --all` and `okfw validate --okf` pass.

## Resolution
Done on 2026-11-19 in Phase 1.

- `okfw init` made `.okfw/`. `okfw install --platform claude` wrote `.claude/skills/okfw/SKILL.md` and the okfw block of `CLAUDE.md`.
- `.mcp.json` registers `okfw mcp`. `.gitignore` ignores only the local settings of `.claude/`.
- The wiki has the two guides, the architecture overview, four specs, ADR-0001 to ADR-0009 and ten phase issues.
- `vocabulary.cue` holds the project terms. lexis reports zero findings on the wiki, `CLAUDE.md` and `PLAN.md`.
- `okfw validate --all` and `okfw validate --okf` pass.

## Related
- [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)
- [Development workflow](/development-workflow--01a1f5379dcf.md)
