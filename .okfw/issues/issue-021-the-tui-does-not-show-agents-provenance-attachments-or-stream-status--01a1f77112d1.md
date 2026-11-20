---
id: "01a1f771-12d1-7d27-8f8d-a1921bf7db81"
title: "ISSUE-021: The TUI does not show agents, provenance, attachments or stream status"
type: "Issue"
kind: "issue"
tags:
  - "issue"
  - "phase-15"
  - "chit-tui"
  - "open"
created: "2026-11-19T23:32:25-06:00"
timestamp: "2026-11-19T23:32:25-06:00"
description: "A post that an agent makes for a person looks like the person typed it, attachments are invisible, and the TUI ignores stream_status."
---

# ISSUE-021: The TUI does not show agents, provenance, attachments or stream status

**Status/Phase:** Open · Phase 15

## Description
The TUI shows agents and bots as people. It does not show the on-behalf-of actor or the provenance of a post. A post with only attachments has an empty body. The TUI ignores the stream_status event, so the user does not see that an agent writes a reply.

## Acceptance Criteria
- [ ] The post header shows the agent or bot badge, the via actor and the provenance badge.
- [ ] Each attachment has a line under the post.
- [ ] The TUI gets file information in batches that obey the rate limit.
- [ ] stream_status shows an indicator above the input.
- [ ] A detail overlay shows the provenance and the files of a post.
- [ ] make check passes.
