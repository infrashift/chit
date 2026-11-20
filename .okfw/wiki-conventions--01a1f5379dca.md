---
id: "01a1f537-9dca-7204-9e54-a433b14872bb"
title: "Wiki conventions"
type: "Guide"
tags:
  - "conventions"
  - "meta"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "Layout, writing style and authoring rules for architecture notes, specs, ADRs, issues and daily notes in this wiki."
---

# Wiki conventions

This guide tells how the chit OKF wiki (`.okfw/`) keeps project knowledge.

- Make concepts with `okfw create` or the okfw MCP tool `okfw_create`. Do not write frontmatter by hand.
- Search before you write. Link to related concepts.
- Keep one idea in each concept. Give each concept a `description` of one sentence.
- Do not start a `--body` with `# Title`. okfw writes the H1 from the title.

## Layout
| Path | Type | Contents |
|---|---|---|
| `architecture/` | `Architecture` | System overview, module boundaries, data flow. |
| `specs/` | `Spec` | Contracts: REST API, WebSocket events, slash commands, on-behalf-of requests. The spec states the intent. The code obeys it. |
| `decisions/` | `ADR` | Architecture decision records. |
| `issues/` | `Issue` (kind `issue`) | Tracked work: one issue for each `PLAN.md` phase, and bugs and design debt. |
| `daily/` | daily note | `okfw daily`: what changed, what you learned, what is next. |

## PLAN.md and issues
`PLAN.md` is the phase checklist. Each phase has a mirror issue. Each phase heading in `PLAN.md` has a `Tracking:` line with a link to its issue. When you complete a phase, tick the `PLAN.md` boxes and the acceptance criteria of the issue.

## ADRs (`decisions/`)
- Title: `ADR-NNNN: <decision>`. Give the ADRs numbers in sequence. Do not use a number again.
- Command: `okfw create "ADR-NNNN: <title>" --path decisions --type ADR --tags adr,<topic> -d "<one sentence>" --body "<markdown>"`
- Body: **Status** (Proposed, Accepted, or Superseded by ADR-XXXX), **Date**, **Source**, then **Context**, **Decision**, **Consequences**, **Alternatives considered**.
- To replace an ADR, write a new ADR with a link to the old ADR. Change the Status line of the old ADR. Run `okfw set-lifecycle <id> deprecated`.
- Write an ADR before you add a new Go module dependency.

## Issues (`issues/`)
- Title: `ISSUE-NNN: <summary>`. Give the issues numbers in sequence.
- Command: `okfw create "ISSUE-NNN: <title>" --kind issue --path issues --type Issue --tags issue,open,phase-<n>,<area> -d "<summary>" --body "<markdown>"`
- Body: a **Status/Phase** line, **Description**, **Acceptance Criteria** (check boxes), **Notes**, **Related**.
- okfw has no status field for issues. The tags hold the state: `open`, then `in-progress`, then `closed`. Use `okfw remove-tags` and `okfw add-tags`. Keep the Status line the same as the tags. Add a **Resolution** section when you close an issue.
- To find open work, run `okfw search --path issues --tag open`.

## Writing style
All documentation obeys ASD-STE100 Simplified Technical English. This includes the wiki, the docs site, `README.md`, `PLAN.md` and `CLAUDE.md`. The check is lexis ([ADR-0005](/decisions/adr-0005-lexis-checks-the-documentation-with-a-repository-vocabulary--01a1f5379e0c.md)).

- Use the active voice and the simple present tense: "chitd refuses the request."
- Start each step of a procedure with a verb in the imperative: Run, Read, Examine, Add, Set.
- Write 20 words or fewer in a step. Write 25 words or fewer in a descriptive sentence.
- Use a verb, not a noun form of the verb: "chitd checks the header", not "A check of the header occurs".
- Use one term for one concept. This wiki uses these terms:
  - *chitd*: the chat server. *chit-tui*: the terminal client. *chit-mcp*, *chit-claude* and *chit-reconcile*: the other binaries.
  - *person*: a human with a Kratos identity. *machine actor*: an agent or bot with a Hydra OAuth2 client.
  - *user*: the Chit record of a person or a machine actor. Each user has an `actor_type`.
  - *team*, *channel*, *post*, *reply*, *thread*, *tag*, *member*.
  - *on-behalf-of request*: a request where a machine actor acts as a person ([On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md)).
  - *event*: a WebSocket message from chitd. *envelope*: the thin pub/sub message on `chit_events`.
  - *gate*: one check of `make check`. *phase*: one unit of work in `PLAN.md`.
- Do not use the words `should`, `will`, `would`, `please`, `simply`, `just`, `very`, `really` or `easily`. Do not use marketing words.
- Run `make docs-lexis`. Each finding fails the gate.

## Housekeeping
- Use `okfw move` and `okfw delete`. Do not use `mv` or `rm`. The okfw commands repair the links and update the `index.md` lists.
- `status` is the OKF lifecycle field (`draft`, `stable`, `deprecated`). It is not the task state. Mark old knowledge `deprecated`. Do not delete it.
- Do not edit `index.md` or `log.md`. okfw makes them.
- Run `make wiki` before you commit changes to the wiki.

See [Development workflow](/development-workflow--01a1f5379dcf.md).
