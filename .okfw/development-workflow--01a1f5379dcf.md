---
id: "01a1f537-9dcf-7878-973a-3b4821d16f6e"
title: "Development workflow"
type: "Guide"
tags:
  - "conventions"
  - "workflow"
  - "meta"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "The loop for each chit phase: implement, lint, verify until green, update the wiki, then commit."
---

# Development workflow

Each change obeys this loop. Do not report or commit work while a gate fails.

1. **Find the phase.** Read `PLAN.md`. Take the first phase that has an empty box. Run `okfw search --path issues --tag open` to find its issue.
2. **Tag the issue.** Run `okfw remove-tags <id> open && okfw add-tags <id> in-progress`.
3. **Do only that phase.** Keep the layers: API, then App, then Store ([chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)).
4. **Obey the Power of 10 rules.** Read [ADR-0002](/decisions/adr-0002-adapt-the-nasa-power-of-10-rules-to-chit--01a1f5379df7.md). Use loops with a bound and no recursion.
5. **Lint.** Run `make lint lint-tui semscope`. golangci-lint and semscope must report zero findings ([ADR-0003](/decisions/adr-0003-semscope-checks-the-power-of-10-rules--01a1f5379dfe.md), [ADR-0004](/decisions/adr-0004-golangci-lint-enforces-the-power-of-10-linters--01a1f5379e05.md)).
6. **Verify.** Run `make test test-tui`. Run `make test-integration` when you change the store, pub/sub or the bridge.
7. **Do the loop again until all gates pass.** Fix each failure and run all gates again. Do not weaken or delete tests.
8. **Update the wiki during the work.** Record each decision in an ADR when you make it. Write an issue for each bug or design debt.
9. **Close the phase.** Tick the `PLAN.md` boxes and the acceptance criteria. Add a **Resolution** section. Change the tag `in-progress` to `closed`.
10. **Update the docs site.** Add or change the pages in `docs/` for the changed features.
11. **Check the style.** Run `make docs-lexis`. Fix each finding ([ADR-0005](/decisions/adr-0005-lexis-checks-the-documentation-with-a-repository-vocabulary--01a1f5379e0c.md)).
12. **Check the wiki.** Run `make wiki`.
13. **Write the journal.** Add an entry to the `okfw daily` note of today: what changed, what you learned, what is next.
14. **Commit.** Commit the code, the wiki and the docs together. Make one commit for each phase.

`make check` runs all of the gates.

See [Wiki conventions](/wiki-conventions--01a1f5379dca.md).
