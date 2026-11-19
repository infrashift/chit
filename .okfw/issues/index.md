# issues

<!-- okfw:generated — maintained by okfw; edits here are overwritten -->

## Concepts

* [ISSUE-001: OKF wiki and MCP server](issue-001-okf-wiki-and-mcp-server--01a1f5379e2f.md) - chit has an okfw wiki with the zmake layout and the okfw MCP server in .mcp.json.
* [ISSUE-002: Lint gates for the Power of 10 rules and the documentation](issue-002-lint-gates-for-the-power-of-10-rules-and-the-documentation--01a1f5379e37.md) - make check runs golangci-lint, semscope, the tests, lexis and the wiki checks, with the unfixed rules omitted.
* [ISSUE-003: R7: check each return value](issue-003-r7-check-each-return-value--01a1f5379e3e.md) - No discarded call result and no dropped error in either Go module.
* [ISSUE-004: R1: no recursion](issue-004-r1-no-recursion--01a1f5379e46.md) - Remove the two recursion cycles in chit-tui.
* [ISSUE-005: R2: loops with a bound](issue-005-r2-loops-with-a-bound--01a1f5379e4d.md) - Each for loop states its exit condition or its maximum count.
* [ISSUE-006: R9: no function types in chit code](issue-006-r9-no-function-types-in-chit-code--01a1f5379e55.md) - Replace function-typed fields and dispatch tables with interfaces or switch statements.
* [ISSUE-007: R6: no mutable package variables](issue-007-r6-no-mutable-package-variables--01a1f5379e5d.md) - Move or remove package variables, and suppress only the kinds that ADR-0002 allows.
* [ISSUE-008: R4: functions of 60 lines or fewer](issue-008-r4-functions-of-60-lines-or-fewer--01a1f5379e65.md) - Split each function that has more than 60 lines.
* [ISSUE-009: R5: assertion density](issue-009-r5-assertion-density--01a1f5379e6c.md) - Add internal/assert to each module and reach an average of two assertions for each function.
* [ISSUE-010: ASD-STE100 documentation](issue-010-asd-ste100-documentation--01a1f5379e74.md) - The Markdown documentation and the wiki give zero lexis findings.

