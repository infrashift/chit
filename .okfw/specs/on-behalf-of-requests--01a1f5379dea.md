---
id: "01a1f537-9dea-7ebd-a5d1-1ce765a9724b"
title: "On-behalf-of requests"
type: "Spec"
tags:
  - "spec"
  - "auth"
  - "on-behalf-of"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "An allowlisted machine actor sends X-On-Behalf-Of on an allowlisted route to act as a person, and chitd provisions a person it has not seen."
---

# On-behalf-of requests

A machine actor can act as a person. halod, the HALO service, uses this to make a Conversation (a private channel) for a person who has not opened chit.

## Header
`X-On-Behalf-Of` names the person in one of two forms:
- a chit user ID;
- `kratos:<id>`, where `<id>` is a Kratos identity ID in canonical form (a lowercase UUID with hyphens).

## Rules
- The client must be in `CHIT_ON_BEHALF_OF_CLIENTS`. A client that is not in the list gets 403. A person's session with the header gets 403.
- The route must be in the allowlist: `POST /posts`, `POST /channels/{id}/stream_status`, `POST /tags`, `POST /posts/{id}/tags`, `POST /channels/{id}/tags`, `POST /channels`, `POST /channels/{id}/members`, `GET /users/me`.
- For `kratos:<id>`, chitd checks the form before it asks Kratos. It then provisions the person on the same path as a browser request. Provisioning writes an `on_behalf_of` audit record with the action `user_provisioned`.
- chitd refuses with 400: a malformed value, an identity that Kratos does not know, an inactive identity and a deactivated chit user.
- `GET /users/me` with the header returns the person. Without the header, it returns the machine actor.

## Usernames
When the identity has no `username` trait, chitd makes the username from the email (`internal/app/username.go`). It takes the local part and makes it lowercase. It replaces other characters with `-` and cuts the name to fit 64 characters. Then it adds `-` and the first 8 hex digits of the Kratos ID. If another user has the name, chitd tries 12, 16 and then 32 digits. chitd refuses an identity without an email.

## Related
- [ADR-0009](/decisions/adr-0009-allowlisted-machine-actors-act-on-behalf-of-a-person--01a1f5379e28.md)
- [REST API contract](/specs/rest-api-contract--01a1f5379dda.md)
