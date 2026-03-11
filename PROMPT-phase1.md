# PROMPT-phase1.md

Review the attached PRD-Phase1.md and SPECS-Phase1.md.

Your Mission: Implement Phase 1 following a strict Test-Driven Development (TDD) cycle.

Mandatory Workflow for every feature:

    Plan: Create a task list for the feature.

    Red: Write failing tests for the feature using teatest for the TUI components.

    Green: Implement the minimum code to pass tests.

    Verify: Run go test -v ./... and golangci-lint run. You must not proceed to the next feature until linting passes and test coverage for the new code is >90%.

    Refactor: Clean up code and re-verify.

Start by outlining the implementation plan for the first sub-task.