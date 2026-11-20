# Chit-TUI Project Rules

## Build and Quality Commands
- Full Suite: `make all` (lint, test, build)
- Build: `make build`
- Test: `make test` (runs with `-race`)
- Coverage: `make cover`
- Lint: `make lint`
- Clean: `make clean`

## Standard Workflow
- **Verify gate:** ALWAYS run `make lint` and `make test` before you declare a feature complete.
- **TDD rule:** Follow Red-Green-Refactor cycles. Each new feature must include tests in the same PR.
- **Coverage gate:** Keep the test coverage of all new logic above 90%. Run `make cover` to verify.
- **Style:** Use `lipgloss` for all UI. Do not hardcode ANSI colors.

## UI Architecture
- Enforce the Elm Architecture (Model, Update, View).
- Use `teatest` for functional TUI testing. Put each flow that runs through a real Bubble Tea program in `internal/tui/flow_test.go`. Unit tests may call `Update` directly. Use `wantMsg`/`messagesOf` (app_test.go) to check what a returned command does, not only that the command exists.
- The code of the root model is in `internal/tui/*.go`, with one file for each concern (see the package doc in `doc.go`). Put each new handler beside the handlers that it resembles. Do not make `update.go` larger.

## Tests
- Every test must be able to fail: assert on state or on the messages that a command produces.
- Tests must not touch the real home directory. In `TestMain`, the `tui`, `config` and `cmd/chit-tui` packages point `HOME`, `XDG_CONFIG_HOME` and `CHIT_CONFIG_FILE` at a temporary directory. Do the same in each new package that reads or writes config, sessions or themes.

## TODO
- `TODO.md` lists only open work. When you complete an item, lint, test, fix and refactor the code. Then remove the item from `TODO.md`. Record the change in `CHANGELOG.md`. The older lists of completed work are in `docs/history/`.