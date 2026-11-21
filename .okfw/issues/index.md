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
* [ISSUE-011: The TUI acceptance tutorial and README describe the old repository layout](issue-011-the-tui-acceptance-tutorial-and-readme-describe-the-old-repository-layout--01a1f5750c6d.md) - The TUI acceptance tutorial describes a separate chit-tui repository, and the README file tree omits newer files.
* [ISSUE-012: ValidateTagName cuts the reserved prefix at the wrong byte](issue-012-validatetagname-cuts-the-reserved-prefix-at-the-wrong-byte--01a1f5857d5f.md) - A profile/ tag name that spells the prefix with a character whose lowercase is shorter, for example İ, keeps the slash after the cut, and chitd refuses it.
* [ISSUE-013: The input cursor of chit-tui never blinks](issue-013-the-cursor-stops-blinking-after-a-mention-pick--01a1f5ad4c0f.md) - The root model of chit-tui drops each cursor blink message, so the input cursor never blinks; the blink commands of the input are dead code.
* [ISSUE-014: Merge main and bring its new code under the gates](issue-014-merge-main-and-bring-its-new-code-under-the-gates--01a1f5dc77bc.md) - Merge origin/main (attachments, the membership hook, the rate limits) and make its new code and docs pass every gate before the pull request.
* [ISSUE-015: make kube-up waits forever on an old PostgreSQL volume](issue-015-make-kube-up-waits-forever-on-an-old-postgresql-volume--01a1f6332767.md) - make kube-up and make uat-up hang without an error when the PostgreSQL volume is from an older image that runs as another UID.
* [ISSUE-016: A duplicate team or channel name gives a 500](issue-016-a-duplicate-team-or-channel-name-gives-a-500--01a1f63e23c0.md) - Creating a team, or a channel in a team, with a name that already exists gives a 500 instead of a 409, because the store does not map the unique violation.
* [ISSUE-017: The e2e script fails on a used PostgreSQL volume](issue-017-the-e2e-script-fails-on-a-used-postgresql-volume--01a1f751fa53.md) - tests/e2e/test_e2e.sh passes only on a new chit-pgdata volume, because scenarios 10 to 12 make people, a team and channels with names that a previous run already used.
* [ISSUE-018: The TUI client does not model the HALO server contract](issue-018-the-tui-client-does-not-model-the-halo-server-contract--01a1f77112b4.md) - The TUI models, API client and errors do not know the attachments, provenance, actor types, tag routes, Retry-After or the refusal reason of the HALO server.
* [ISSUE-019: The TUI has no reusable Bubbles for confirm, chips, prompt, activity and file pick](issue-019-the-tui-has-no-reusable-bubbles-for-confirm-chips-prompt-activity-and-file-pick--01a1f77112be.md) - The TUI keeps a confirmation, a chip row and other small UI parts inside larger models, so new features cannot use them again.
* [ISSUE-020: A theme change does not restyle all of the TUI, and edited themes need a restart](issue-020-a-theme-change-does-not-restyle-all-of-the-tui-and-edited-themes-need-a-restart--01a1f77112c7.md) - After /theme some parts keep the old colors, an edited theme file needs a restart, and the theme picker has no preview.
* [ISSUE-021: The TUI does not show agents, provenance, attachments or stream status](issue-021-the-tui-does-not-show-agents-provenance-attachments-or-stream-status--01a1f77112d1.md) - A post that an agent makes for a person looks like the person typed it, attachments are invisible, and the TUI ignores stream_status.
* [ISSUE-022: The TUI does not manage tags as the HALO server permits](issue-022-the-tui-does-not-manage-tags-as-the-halo-server-permits--01a1f77112db.md) - The TUI ignores tag_updated, tag_deleted and channel_tags_updated, and it cannot rename or delete a tag or use channel tags.
* [ISSUE-023: The TUI cannot attach or save files](issue-023-the-tui-cannot-attach-or-save-files--01a1f77112e5.md) - The TUI cannot upload an image to a post or save the attachment of a post to disk.
* [ISSUE-024: The TUI e2e suite and the live run do not cover the HALO features](issue-024-the-tui-e2e-suite-and-the-live-run-do-not-cover-the-halo-features--01a1f77112ef.md) - The TUI e2e suite tests none of the HALO routes and events, and no live run checks the TUI against the full stack.

