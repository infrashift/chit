---
id: "01a1f537-9de5-78fc-9859-9b01619c308e"
title: "Slash commands"
type: "Spec"
tags:
  - "spec"
  - "commands"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "CUE files define the slash commands, Keto authorizes each command, and chitd audits each one and replies with a command_response event."
---

# Slash commands

A post that starts with `/` is a slash command. `internal/app/command.go` stops the post before the store gets it.

## Flow
1. The parser (`internal/command/parser.go`) reads the name and the arguments.
2. The CUE definitions in `auth/` give the known commands. `chit-reconcile` writes them to Keto.
3. Keto checks the permission in the namespace `chit/command`.
4. The handler runs and writes an audit record.
5. chitd sends the reply as a `command_response` event to the caller only ([WebSocket events](/specs/websocket-events--01a1f5379de0.md)).

## Handlers
`/help`, `/invite`, `/kick` and `/topic` have handlers. `/summarize` is in the CUE files but has no handler yet.

## Source
`SPECS-SLASH-COMMANDS.md` and `PRD-SLASH-COMMANDS.md` hold the full design.

## Related
- [chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)
- [ADR-0006](/decisions/adr-0006-the-ory-stack-authenticates-and-chitd-authorizes--01a1f5379e13.md)
