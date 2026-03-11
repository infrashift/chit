# Chit-TUI Project Rules

## Build and Quality Commands
- Full Suite: `make all` (lint, test, build)
- Build: `make build`
- Test: `make test`
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
- Use `teatest` for functional TUI testing.

## TODO
- Maintain a TODO.md for each business-valued feature. When you complete the feature, lint, test, and fix and refactor, update the TODO.