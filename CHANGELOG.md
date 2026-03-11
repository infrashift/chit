# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Layered architecture scaffold: API → App → Store → PostgreSQL
- PostgreSQL schema with UUIDv7 primary keys and bigint millisecond timestamps
  (10 tables: users, teams, team_members, channels, channel_members, posts,
  threads, thread_memberships, tags, message_tags)
- OpenAPI 3.1 specification with 40+ endpoints across 9 resource tags
- Podman Kube deployment manifest with 5 containers (PostgreSQL, Kratos,
  Oathkeeper, Keto, ZincSearch)
- Ory stack configuration files (Kratos identity schemas, Oathkeeper access
  rules, Keto namespace configuration)
- WebSocket hub with per-user connection map and single event loop goroutine
- Pub/sub abstraction with PG LISTEN/NOTIFY backend (NATS optional)
- LRU cache layer using hashicorp/golang-lru/v2
- ZincSearch indexer skeleton
- MCP server for AI agent integration
- Containerfile for building the server image
- Makefile with build, test, lint, migrate, and kube targets
- Environment-based configuration via koanf with CHIT_ prefix (23 variables)
- Documentation site built with Astro + Starlight
- README, ROADMAP, and CHANGELOG
