---
id: "01a1f537-9dda-7bcf-97d6-eb99d021c124"
title: "REST API contract"
type: "Spec"
tags:
  - "spec"
  - "api"
created: "2026-11-19T13:10:25-06:00"
timestamp: "2026-11-19T13:10:25-06:00"
description: "api/openapi.yaml is the contract of the chitd REST API under /api/v1; the router and the spec change together."
---

# REST API contract

`api/openapi.yaml` is an OpenAPI 3.1 document. It is the contract of the chitd REST API. `api/embed.go` embeds it, and chitd serves it with a Swagger UI.

## Rules
- All routes are under `/api/v1`. `GET /api/v1/system/ping` is the health check.
- Oathkeeper authenticates each route except the ping ([chit architecture overview](/architecture/chit-architecture-overview--01a1f5379dd5.md)).
- Change the router in `internal/api/api.go` and the spec in the same commit. `make validate-spec` checks the spec.
- A 5xx response contains no internal detail.
- A list route has a bounded page size.
- A route that accepts `X-On-Behalf-Of` is in the allowlist of [On-behalf-of requests](/specs/on-behalf-of-requests--01a1f5379dea.md), in `internal/api/middleware.go` and in `api/openapi.yaml`.

## Resource tags
The spec groups more than 40 endpoints in tags: system, users, teams, channels, posts, threads, tags, search, commands, websocket.

## Related
- [WebSocket events](/specs/websocket-events--01a1f5379de0.md)
- [Slash commands](/specs/slash-commands--01a1f5379de5.md)
