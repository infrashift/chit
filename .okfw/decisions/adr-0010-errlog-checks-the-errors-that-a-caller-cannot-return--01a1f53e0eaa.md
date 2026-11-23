---
x-okfw-id: "01a1f53e-0eaa-773d-a97a-f9f1fbcc3194"
title: "ADR-0010: errlog checks the errors that a caller cannot return"
type: "ADR"
tags:
  - "adr"
  - "power-of-10"
  - "r7"
  - "errors"
x-okfw-created: "2026-11-19T13:17:27-06:00"
x-okfw-timestamp: "2026-11-19T13:17:27-06:00"
description: "Each module has internal/errlog, which logs with slog the errors of deferred closes and best-effort cleanups, so no error goes to the blank identifier."
---

# ADR-0010: errlog checks the errors that a caller cannot return

**Status:** Accepted
**Date:** 2026-11-19
**Source:** [ISSUE-003](/issues/issue-003-r7-check-each-return-value--01a1f5379e3e.md). semscope and golangci-lint with `check-blank` report each `_ = f()`.

## Context
R7 of [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md) requires a check of each return value. chit had 82 R7 findings from semscope and 225 errcheck findings from golangci-lint ([ADR-0004](/decisions/adr-0004-golangci-lint-enforces-the-power-of-10-linters--01a1f5379e05.md)). Most of them were a deferred `Close`, a cleanup after a failure, or a write to a peer that left. The caller cannot return such an error and cannot act on it. Holzmann allows a cast to `void` for such a call, but semscope reports each `_ =` as a finding.

## Decision
- Each Go module has `internal/errlog` with three functions: `Close(c, what)`, `Debug(err, what)` and `Warn(err, what)`. They log a failure with slog.
- Use `defer errlog.Close(resp.Body, "keto response body")` for a deferred close. Do not write `defer errlog.Debug(x.Close(), ...)`, because Go runs `Close` at once.
- An HTTP response body is the exception. The `bodyclose` linter does not see `errlog.Close(resp.Body, ...)`, so use `defer func() { errlog.Debug(resp.Body.Close(), "...") }()`.
- Use the result when the caller can act on it: return it, wrap it with `%w`, or branch on it. Name the `found` result of `strings.Cut` and use it.
- A failure path that cleans up joins the cleanup errors: `errors.Join(err, tmp.Close(), os.Remove(name))`.
- A test checks each error with `t.Fatalf`, or with `t.Errorf` in another goroutine.
- `hash.Hash.Write` never returns an error, but semscope sees `(io.Writer).Write`. The caller checks it with `errlog.Warn`. An ignore entry for `(io.Writer).Write` covers too many calls.

## Consequences
- No error goes to the blank identifier in either module.
- A failed close is visible at the debug level of slog.

## Alternatives considered
- **A suppression on each site.** Rejected: more than 200 comments with the same reason.
- **Add the calls to `go_ignore_calls`.** Rejected: a close can fail, so the list is for calls that never fail.
