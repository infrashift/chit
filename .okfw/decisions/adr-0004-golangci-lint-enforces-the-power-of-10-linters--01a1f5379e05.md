---
id: "01a1f537-9e05-70e2-8f58-e5b17773cf31"
title: "ADR-0004: golangci-lint enforces the Power of 10 linters"
type: "ADR"
tags:
  - "adr"
  - "lint"
  - "power-of-10"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Both .golangci.yml files enable funlen, gochecknoglobals, forbidigo, errcheck check-blank and the other linters that match the Power of 10 rules."
---

# ADR-0004: golangci-lint enforces the Power of 10 linters

**Status:** Accepted
**Date:** 2026-11-19
**Source:** User request: lint with the Go linter as well. semscope uses the same linters in its `.golangci.yml`.

## Context
CI runs golangci-lint on both Go modules. CI cannot run semscope ([ADR-0003](/decisions/adr-0003-semscope-checks-the-power-of-10-rules--01a1f5379dfe.md)). Some golangci-lint linters check a part of the Power of 10 rules, so CI can check those parts.

## Decision
Both `.golangci.yml` files enable these linters. They keep the linters that are on now.

| Linter | Rule |
|---|---|
| `funlen` (60 lines) | R4 |
| `gochecknoglobals` | R6 |
| `forbidigo` (`panic` only in `internal/assert`) | R5, R7 |
| `errcheck` with `check-blank` and `check-type-assertions` | R7 |
| `errorlint` | R7 |
| `gocognit`, `nestif` | R1 |
| `depguard` (no `unsafe`), `gosec` | R10 |

- A phase enables a linter when it removes the omission of the semscope rule for it. `gocognit` and `nestif` are the exception: Phase 8 (R4) enables them, because they report the same long functions as R4.
- `make lint-tui` uses the same `GOLANGCI_LINT` command as `make lint`.

## Consequences
- CI checks R4, R6 and R7 without semscope.
- `gochecknoglobals` and semscope need a suppression on the same line for a package variable that stays: `//nolint:gochecknoglobals // semscope: allow(NASA-P10-06) <reason>`.
