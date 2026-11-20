# PLAN.md — chit

This file is the phase checklist. Each phase has a mirror issue in `.okfw/issues/`. The *Development workflow* guide in `.okfw/` gives the loop for each phase.

## Phase 1: OKF wiki and MCP server
Tracking: [ISSUE-001](.okfw/issues/issue-001-okf-wiki-and-mcp-server--01a1f5379e2f.md)

- [x] Run `okfw init` and `okfw install --platform claude`.
- [x] Add `.mcp.json` with `okfw mcp`.
- [x] Write the guides, the architecture overview, the specs and ADR-0001 to ADR-0009.
- [x] Write `CLAUDE.md`, `PLAN.md` and `vocabulary.cue`.

## Phase 2: Lint gates
Tracking: [ISSUE-002](.okfw/issues/issue-002-lint-gates-for-the-power-of-10-rules-and-the-documentation--01a1f5379e37.md)

- [x] Add `.semscope.cue` with the R3 deviation and the omissions.
- [x] Add the `semscope`, `docs-lexis`, `wiki` and `check` targets.
- [x] Fix the golangci-lint issues of the baseline (R10).

## Phase 3: R7, check each return value
Tracking: [ISSUE-003](.okfw/issues/issue-003-r7-check-each-return-value--01a1f5379e3e.md)

- [x] Fix each discarded result and each dropped error.
- [x] Remove the NASA-P10-07 omission. Turn on `errcheck` `check-blank` and `errorlint`.

## Phase 4: R1, no recursion
Tracking: [ISSUE-004](.okfw/issues/issue-004-r1-no-recursion--01a1f5379e46.md)

- [x] Remove the two recursion cycles in chit-tui.
- [x] Remove the NASA-P10-01 omission. Move `gocognit` and `nestif` to Phase 8.

## Phase 5: R2, loops with a bound
Tracking: [ISSUE-005](.okfw/issues/issue-005-r2-loops-with-a-bound--01a1f5379e4d.md)

- [x] Replace each `for {}` with an exit condition or a maximum count.
- [x] Remove the NASA-P10-02 omission.

## Phase 6: R9, no function types
Tracking: [ISSUE-006](.okfw/issues/issue-006-r9-no-function-types-in-chit-code--01a1f5379e55.md)

- [x] Replace the fields of function type and the dispatch tables.
- [x] Suppress the function types that a framework requires.
- [x] Remove the NASA-P10-09 omission.

## Phase 7: R6, no mutable package variables
Tracking: [ISSUE-007](.okfw/issues/issue-007-r6-no-mutable-package-variables--01a1f5379e5d.md)

- [x] Move or remove the package variables.
- [x] Remove the NASA-P10-06 omission. Turn on `gochecknoglobals`.

## Phase 8: R4, functions of 60 lines or fewer
Tracking: [ISSUE-008](.okfw/issues/issue-008-r4-functions-of-60-lines-or-fewer--01a1f5379e65.md)

- [x] Split each long function.
- [x] Remove the NASA-P10-04 omission. Turn on `funlen`, `gocognit` and `nestif`.

## Phase 9: R5, assertion density
Tracking: [ISSUE-009](.okfw/issues/issue-009-r5-assertion-density--01a1f5379e6c.md)

- [x] Add `internal/assert` to each module.
- [x] Add assertions to an average of two for each function.
- [x] Add recover guards to the goroutines.
- [x] Remove the NASA-P10-05 omission. Turn on `forbidigo`.

## Phase 10: ASD-STE100 documentation
Tracking: [ISSUE-010](.okfw/issues/issue-010-asd-ste100-documentation--01a1f5379e74.md)

- [x] Write the Markdown documentation again until `make docs-lexis` reports zero findings.

## Phase 11: Merge main and bring its new code under the gates
Tracking: [ISSUE-014](.okfw/issues/issue-014-merge-main-and-bring-its-new-code-under-the-gates--01a1f5dc77bc.md)

- [x] Merge origin/main and resolve the conflicts.
- [x] Make the new code pass semscope and golangci-lint, with assertions.
- [x] Write the new docs in STE, and update the wiki.
- [x] Run every gate and every e2e suite, then push and open the pull request.

## Phase 12: The TUI contract layer
Tracking: [ISSUE-018](.okfw/issues/issue-018-the-tui-client-does-not-model-the-halo-server-contract--01a1f77112b4.md)

- [x] Add the HALO fields and types to the TUI models.
- [x] Add Reason, Detail and RetryAfter to APIError, and add UserMessage.
- [x] Add the file, tag and channel-tag methods to the API client and to the test doubles.
- [x] Add ValidTagName, and accept Unicode letters in a hashtag.

## Phase 13: Reusable Bubbles
Tracking: [ISSUE-019](.okfw/issues/issue-019-the-tui-has-no-reusable-bubbles-for-confirm-chips-prompt-activity-and-file-pick--01a1f77112be.md)

- [x] Add the packages `ui/confirm`, `ui/chips`, `ui/prompt`, `ui/activity` and `ui/filepick`.
- [x] Move the post delete and the member picker to the new packages.
- [x] Record the contract of a Bubble in an ADR.

## Phase 14: Runtime themes
Tracking: [ISSUE-020](.okfw/issues/issue-020-a-theme-change-does-not-restyle-all-of-the-tui-and-edited-themes-need-a-restart--01a1f77112c7.md)

- [x] Make one restyle path, and test that a theme change equals a fresh start.
- [x] Add the preview in the picker, /theme <name> and /theme reload.
- [x] Add the theme-file watch and the theme_watch setting.
- [x] Tell users about /theme in the help overlay and the docs.

## Phase 15: Show the HALO data
Tracking: [ISSUE-021](.okfw/issues/issue-021-the-tui-does-not-show-agents-provenance-attachments-or-stream-status--01a1f77112d1.md)

- [x] Show the actor, via and provenance badges, and the attachment lines.
- [x] Get file information in batches that obey the rate limit.
- [x] Show stream_status above the input.
- [x] Add the post detail overlay.

## Phase 16: Tag parity
Tracking: [ISSUE-022](.okfw/issues/issue-022-the-tui-does-not-manage-tags-as-the-halo-server-permits--01a1f77112db.md)

- [x] Handle tag_updated, tag_deleted and channel_tags_updated.
- [x] Rename and delete a tag from the tag picker.
- [x] Show the channel tags, and add /chtag.

## Phase 17: Attachments
Tracking: [ISSUE-023](.okfw/issues/issue-023-the-tui-cannot-attach-or-save-files--01a1f77112e5.md)

- [x] Add /attach, the file tray and a post with files.
- [x] Save the files of a post to download_dir.

## Phase 18: Verify the TUI against the HALO stack
Tracking: [ISSUE-024](.okfw/issues/issue-024-the-tui-e2e-suite-and-the-live-run-do-not-cover-the-halo-features--01a1f77112ef.md)

- [ ] Add the TUI e2e tests for the HALO routes and events.
- [ ] Do a live run through Oathkeeper.
- [ ] Update the verify skill, the UAT tutorial and the changelogs.
