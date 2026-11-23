---
x-okfw-id: "01a1f537-9e21-74f5-943a-c5a8f51d102f"
title: "ADR-0008: chit-tui is a separate Go module"
type: "ADR"
tags:
  - "adr"
  - "tui"
  - "dependencies"
x-okfw-created: "2026-11-19T13:10:25-06:00"
x-okfw-timestamp: "2026-11-19T13:10:25-06:00"
description: "clients/chit-tui has its own go.mod so the charmbracelet dependencies stay out of the server build, and make check-deps enforces it."
---

# ADR-0008: chit-tui is a separate Go module

**Status:** Accepted
**Date:** 2026-11-19 (records an earlier decision)
**Source:** `README.md`, `Makefile` target `check-deps`.

## Context
chit-tui uses Bubble Tea and other charmbracelet packages. The server does not need them.

## Decision
- `clients/chit-tui` has its own `go.mod`. `go.work` joins the two modules for local work.
- chit-tui talks to chitd over REST and WebSocket only. It does not import the server packages.
- `make check-deps` fails when a charmbracelet module is in the server module graph.

## Consequences
- Each module has its own `internal/assert` package ([ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md)), because Go does not let one module import the `internal/` packages of another module.
- golangci-lint runs once for each module.
