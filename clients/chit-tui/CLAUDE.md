# Chit-TUI Project Rules

## Build and Quality Commands
- Full Suite: `make all` (lint, test, build)
- Build: `make build`
- Test: `make test` (runs with `-race`)
- Coverage: `make cover`
- Lint: `make lint`
- Clean: `make clean`

## Standard Workflow
- **Verification Gate:** ALWAYS run `make lint` and `make test` before declaring a feature complete.
- **TDD Requirement:** Follow Red-Green-Refactor cycles. New features must include tests in the same PR.
- **Coverage Gate:** Maintain >90% test coverage on all new logic. Run `make cover` to verify.
- **Style:** Use `lipgloss` for all UI. Hardcoded ANSI colors are forbidden.

## UI Architecture
- Enforce the Elm Architecture (Model, Update, View).
- Use `teatest` for functional TUI testing: flows that should run through a real Bubble Tea program go in `internal/tui/flow_test.go`. Unit tests may call `Update` directly; use `wantMsg`/`messagesOf` (app_test.go) to check what a returned command does, not merely that one exists.
- The root model is split by concern across `internal/tui/*.go` (see the package doc in `doc.go`); put new handlers beside the ones they resemble rather than growing `update.go`.

## Tests
- Every test must be able to fail: assert on state or on the messages a command produces.
- Tests must not touch the real home directory. The `tui`, `config` and `cmd/chit-tui` packages point `HOME`, `XDG_CONFIG_HOME` and `CHIT_CONFIG_FILE` at a temporary directory in `TestMain`; do the same in any new package that reads or writes config, sessions or themes.

## TODO
- `TODO.md` lists open work only. When you complete an item, lint, test, fix and refactor, then remove it from `TODO.md` and record the change in `CHANGELOG.md`. Older completed lists are in `docs/history/`.