# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Upgrade notes

Behaviour changes an operator must act on or know about:

- **Attachments (migration `000009`) and a SeaweedFS container.** The
  migration adds `files` and `post_files`; its down migration drops them
  and leaves the objects in the bucket. Attachments are off until
  `CHIT_S3_ENDPOINT` is set, with `CHIT_S3_ACCESS_KEY_ID` and
  `CHIT_S3_SECRET_ACCESS_KEY` (chitd will not start with the endpoint and
  no keys). The deploy files add a `seaweedfs` container
  (trusted-service-containers `trusted/seaweedfs:4.51-382cdac`, by digest)
  on 127.0.0.1 ports 8333 (S3), 9333, 8380 and 8888 and gRPC 19333, 18380,
  18888 and 18334, a `chit-seaweedfs-config` ConfigMap with the dev S3 key,
  and a `chit-seaweeddata` volume (`chit-uat-seaweeddata` for UAT), which
  `make kube-clean` and `make uat-clean` now remove. chitd creates the
  `chit-files` bucket at start, waiting up to 60 seconds for the store.
  `CreatePostRequest.content` is no longer required: a post of attachments
  alone has none.
- **Tags have an owner and a scope (migration `000008`).** A tag is now
  `private` (its owner's alone, on anything they can read) or `channel`
  (shared by one channel's members, usable only there), and names are unique
  per owner or per channel after NFKC and lowercasing (ß and ss stay
  distinct), no longer server-wide. Old names that now share a key in one
  channel ("Bug", "bug") merge into one tag.
  The migration turns each existing tag into a channel tag in every channel
  where a post carries it, owned by the new system actor `chit-system`
  (Chit never recorded who created or applied a tag; a system_admin may
  rename or delete these), and drops tags applied nowhere. The down
  migration is lossy: it drops private tags and channel tags and merges
  same-named tags. `POST /tags` without a `scope` now creates a **private**
  tag; chit-tui asks for a channel tag of the current channel, as its
  tags always behaved, and makes a private one only on request (Ctrl+T in
  the tag picker).
  `GET /tags`, post and batch tag reads, search and `post_tags_updated` show
  each user only the tags they can see.
- **Keto is now keto-cue, with a CUE permission schema.**
  [keto-cue](https://github.com/infrashift/keto-cue) is the Ory Keto fork that
  reads CUE instead of OPL; its server, storage and REST API are upstream
  Keto's (master after v26.2.0). `deploy/keto/keto.yml` now points
  `namespaces.location` at the directory `deploy/keto/schema/`, whose
  `chit.cue` declares `chit/channel`, `chit/command` and a subject-only
  `chit/actor`; the `chit-keto-config` and `chit-keto-schema` ConfigMaps
  carry them. keto-cue compiles every file in that directory as one
  package, so a service sharing the instance (HALO) adds one file in
  `package keto` with its own namespace prefix. Tuples, paths and request
  shapes do not change, and `make test-keto` runs the same assertions against
  either server. Two answers differ from v0.12: a write to an undeclared
  namespace is a `404`, and a check of an undeclared relation is a `400`
  (v0.12 said `403`, not allowed). Chit treats both as a denial. Add a
  relation to the schema before Chit writes or checks it, and run
  `make keto-validate`. `keto migrate up` upgrades an existing v0.12
  database in place.
- **Keto tuple writes look before they write.** Keto stores a row per
  write, so every `chit-reconcile` run and every channel re-join used to
  add duplicate rows. The client now lists the exact tuple and writes only
  when it is absent, which also means a write needs the Keto read API
  (`CHIT_KETO_READ_URL`). Duplicates already stored stay, harmlessly; a
  narrow race between two writers can still add one.
- **keto-cue comes from trusted-service-containers.** The deploy files,
  `make keto-validate` and `make test-keto` pull
  `trusted/keto-cue:v0.0.0-20261114211924-809351b32b9c-6a44b89`, pinned by
  digest: keto-cue `809351b3`, built from source on UBI9 Micro. Like the Ory
  images it runs as UID 1001 with `HOME=/home/ory`, and its entrypoint is
  `keto`. There is no local keto-cue build to run first.
- **Ory images moved to trusted-service-containers v26.2.0**: Kratos (from
  v1.3), Hydra (from v2.2) and Oathkeeper (from v0.40), built from source on
  UBI9 Micro and pinned by digest. Every image in the deploy files and
  Makefiles is now pinned by digest. The only configuration change the jump
  needs is the `version:` key (`v26.2.0`); the init containers migrate the
  Kratos and Hydra databases. The images run as UID 1001 with
  `HOME=/home/ory`.
- **PostgreSQL 17 is the trusted-service-containers postgresql17 image on
  UBI, and an existing `chit-pgdata` volume must be dumped and restored.**
  The deploy files, `make test-integration`, `make test-e2e`, the chit-tui
  `make test-e2e` and CI use
  `trusted/postgresql17:17.12-9410cd4`, pinned by digest: the signed PGDG
  RPMs (contrib included, for `pgcrypto`) on trusted UBI9 Minimal. It
  replaces the quarantined mirror of Docker's hardened `postgres:17-debian`.
  The `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` contract and
  port 5432 do not change. The image runs as UID 26 with primary group 0
  and refuses to run as root. The volume now mounts at
  `/var/lib/pgsql/data`, and the cluster lives in
  `PGDATA=/var/lib/pgsql/data/userdata`. New databases use the PostgreSQL
  builtin `C.UTF-8` collation, so text sorts by code point (for example,
  channel and team lists by `display_name` put capitals first).
  A volume from the old image (UID 70, `/var/lib/postgresql/17/data`,
  glibc `en_US.UTF-8`) **does not mount** in the new one, and the new image
  has no `en_US.UTF-8` locale. Before you pull this change, dump the
  cluster with the old image: `podman exec chit-db-postgres pg_dumpall -U
  chit > chit-pgdumpall.sql`, and check that the file ends with
  `PostgreSQL database cluster dump complete`. Then run `make kube-down` and
  `podman volume rm chit-pgdata` (only that volume: `make kube-clean` also
  deletes the search index and the attachments). After the pull, restore
  into a fresh volume on the new image before `make kube-up`. Create the
  `kratos`, `keto` and `hydra` databases first, because `pg_dumpall` names
  the old locale in each `CREATE DATABASE` and those statements fail. The
  restore then prints `role "chit" already exists` and one
  `invalid LC_COLLATE locale name: "en_US.UTF-8"` per database, which are
  expected. "Upgrading from the DHI PostgreSQL image" in
  `docs/.../deployment/podman-kube.mdx` lists the commands. A restore
  rebuilds every index under the new collation, so the `REINDEX` that an
  Alpine volume needed is done too. A volume with nothing to keep, such as
  the UAT stack's `chit-uat-pgdata` or a throwaway dev stack's, must still
  be created again, because the old one does not start: run `make uat-clean` or `make kube-clean` once, and the next
  `make uat-up` or `make kube-up` initialises an empty volume.
- **The `chit-postgres-init` ConfigMap is gone.** The `ensure-databases`
  init container (now in the UAT manifest too) creates the Ory databases on
  every start, which also mends a volume that lacks one. An init script in
  `/docker-entrypoint-initdb.d` would run only on a fresh data directory.
- **`chit-reconcile` now revokes, and needs the Keto read API.** It makes
  the `chit/command` namespace match the CUE, deleting any tuple there the CUE
  does not declare (including grants added to Keto by hand). It lists through
  `CHIT_KETO_READ_URL` (default `http://localhost:4466`); set it wherever the
  job runs.
- **`chit-mcp` and `chit-claude` take exactly one auth mode.** Setting both
  `*_OAUTH_CLIENT_ID` and `*_AGENT_KRATOS_ID` now stops them at startup;
  OAuth2 used to win silently. Unset the one you do not use.
- **`chit-claude` answers only when addressed.** `CHIT_CLAUDE_REQUIRE_MENTION`
  now defaults to `true`: a post must name the agent, after which replies in
  that thread need no mention (`CHIT_CLAUDE_FOLLOW_THREADS`, default `true`).
  `@all` and `@channel` no longer count unless
  `CHIT_CLAUDE_ANSWER_BROADCASTS=true`. Set `REQUIRE_MENTION=false` to keep a
  channel where every post is answered.
- **`chit-claude` runs no longer inherit the host's Claude Code setup.** Runs
  pass `--strict-mcp-config` and `--setting-sources project`: MCP servers,
  permissions, hooks and plugins from the host user's `~/.claude` stop
  applying. Name MCP servers in `CHIT_CLAUDE_MCP_CONFIG`; set
  `CHIT_CLAUDE_SETTING_SOURCES=all` to load every source again.
- **`chit-claude` checks its channels at startup**, and refuses to start when
  chitd says its agent is not a member of one listed in
  `CHIT_CLAUDE_CHANNELS`.
- **`chit-mcp` returns posts without their props** (and without hashtags or
  update/delete times), and `get_new_events` returns only `posted`,
  `post_edited` and `post_deleted` unless `types` names others. An agent that
  read props from tool results must fetch the post over the REST API instead.
- **`actor_type` is required for machine actors.** Every entry in
  `CHIT_MACHINE_ACTORS` must set `actor_type` to `agent` or `bot`; an entry
  without it now fails boot, naming the client. Add it before upgrading.
- **`CHIT_ZINCSEARCH_URL` defaults to empty.** ZincSearch is opt-in: without
  a URL, search runs in PostgreSQL and no indexer runs. Set it explicitly to
  keep using Zinc (both deploy manifests do).
- **Configuration is validated at startup.** Invalid settings (non-positive
  `CHIT_WS_PING_INTERVAL`, `CHIT_WS_WRITE_TIMEOUT`, `CHIT_CACHE_SIZE`,
  `CHIT_CACHE_TTL` or DB pool sizes; an unknown `CHIT_PUBSUB_BACKEND`;
  `nats` without `CHIT_NATS_URL`; `CHIT_WEBHOOK_ENABLED=true` without
  `CHIT_WEBHOOK_URL`; a bad log level or format) now stop the server, all
  reported at once. Some of these were silently ignored before.
- **An empty `CHIT_ALLOWED_ORIGINS` allows no browser origin**, for CORS and
  WebSockets alike. It used to mean "allow all" for CORS. Unset, it still
  defaults to `*`.
- **`POST /channels` creates only open (`O`) and private (`P`) channels** and
  requires `team_id`. Use `POST /channels/direct` and `/channels/group` for
  DMs and groups.
- **Direct and group channel membership is fixed.** Adding a member to one is
  a `403`.
- **`POST /tags` is idempotent**: an existing name returns the existing tag
  (`201`), never `409`.
- **`GET /teams/{id}/channels` lists open channels only.** Private channels
  are listed for their members by `GET /users/me/teams/{id}/channels`.
- **Team reads are authorized.** `GET /teams` lists open teams plus the
  caller's own; invite-only teams and every roster need membership (or
  `system_admin`); `PUT`/`DELETE /teams/{id}` need `team_admin` or
  `system_admin`.
- **Migration `000007`** merges any duplicate direct or group channels into
  the oldest copy before adding a unique index. It is not reversible for
  merged channels.

### Added

- Attachments (P-CHIT-5 for HALO): `POST /channels/{id}/files` uploads an
  image (multipart `file` and `alt_text`), `GET /files/{id}` serves its
  bytes, `GET /files/{id}/info` its metadata, and `POST /files/search`
  finds files by name and alt text only, in the caller's channels.
  `file_ids` on `POST /posts` attaches up to 4 of the author's uploads in
  that channel; posts read back with `file_ids`, and deleting a post
  deletes its files. Uploads are checked by the new package
  `pkg/imagecheck` (no Chit dependencies, for HALO to reuse): PNG, JPEG,
  WebP and GIF (first frame) only, SVG refused, declared and sniffed types
  must agree, polyglots (trailing bytes, markup in metadata) refused,
  `image.DecodeConfig` refuses more than 40 MP before decoding, then a pure
  Go re-encode (JPEG to JPEG, the rest to PNG) drops EXIF and anything
  else; 10 MiB in and out, and `CHIT_FILE_QUOTA_BYTES` (1 GiB) per person.
  Refusals carry an `id` (`file.svg`, `file.polyglot`, ...). Bytes go to
  the S3 bucket `chit-files` under `<channel_id>/<file_id>` through a small
  SigV4 client; identical bytes share one object within a channel, never
  across channels. Before it is attached only the uploader reads a file,
  then every member; a non-member gets 403. Every file response carries
  `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox`
  and `Content-Disposition`. A sweep, at most `CHIT_FILE_SWEEP_BATCH` files
  each `CHIT_FILE_SWEEP_INTERVAL`, deletes uploads never attached and the
  objects of deleted files after `CHIT_FILE_ORPHAN_GRACE` (24 h).
- `X-On-Behalf-Of` on more routes, so halod can manage a person's tags and
  attachments as the person: `GET /tags`, `GET /channels/{id}/tags`,
  `POST /tags/posts`, `POST /tags/channels`,
  `DELETE /posts/{id}/tags/{tag_id}`, `DELETE /channels/{id}/tags/{tag_id}`,
  `POST /channels/{id}/files` (a `file_uploaded` audit record names the
  client) and `GET /files/{id}`. The person sees only their own private
  tags. A test now holds the route list and the spec's header declarations
  to the same set.
- On-behalf-of provisioning and Conversations (P-CHIT-8 for HALO):
  `X-On-Behalf-Of` also takes `kratos:<Kratos identity ID>` (a canonical
  UUID). A person who has never opened Chit gets the user their first
  browser request would make, and a `user_provisioned` audit record names
  the client. The header is now also accepted on `POST /channels` (the
  person must be on the team and becomes the creator), on
  `POST /channels/{id}/members` (the person must be allowed to add a
  member), and on `GET /users/me`, which returns the person; without the
  header a machine actor gets itself, which is how halod learns its user ID.
  `channel_created` and `channel_member_added` audit records name the
  client. Refused: a malformed `kratos:` value, an identity Kratos does not
  know, an inactive identity and a deactivated user (400); a client not on
  the list or a person's session (403).
- A Kratos identity without a `username` trait (HALO's schema has only
  `email`) can be provisioned, from a browser or on behalf of. Its username
  is the email's local part, lowercased, with each run of other characters
  than `a-z 0-9 . _ -` made one `-`, cut to fit, then `-` and the first 8
  hex digits of the Kratos ID (`sam.smith-6f1f2a64`); `user-` and 12 digits
  without a usable email. A taken name takes a longer suffix (12, 16, then
  32 digits). The display name falls back to it. An explicit
  `traits.username` is used as before.
- Scoped tags (P-CHIT-7 for HALO): `scope` and `channel_id` on `POST /tags`;
  `PUT`/`DELETE /tags/{id}` (owner only); tags on channels
  (`POST`/`GET /channels/{id}/tags`, `DELETE /channels/{id}/tags/{tag_id}`,
  `channel_tags` table) with `applied_by` recorded on channel and post
  applications; `POST /tags/channels`, a channel filter by up to 8 tags that
  must all match on the channel or one of its posts; `DELETE /users/{id}/tags`
  for deleting or anonymizing a person; `channel_tags_updated`,
  `tag_updated` and `tag_deleted` events. Limits: 50 characters, 32 tags per
  item, 1024 per owner, 8 per filter. The `profile/` prefix is reserved for
  HALO acting on a person's behalf (`X-On-Behalf-Of` is now also accepted on
  `POST /tags`, `/posts/{id}/tags` and `/channels/{id}/tags`).
- On-behalf-of posting (P-CHIT-1 for HALO): a machine actor listed in
  `CHIT_ON_BEHALF_OF_CLIENTS` may post as a person, and send
  `stream_status` for them, by naming the person's user ID in
  `X-On-Behalf-Of`. The post is authorized as the person and stored as
  theirs; `props.on_behalf_of.actor_id` and a `post_on_behalf_of` audit
  record name the client. A client not on the list, a person's session, and
  an empty, malformed, unknown or non-person principal are refused, and the
  header is refused on every other route.
- `props.provenance` (P-CHIT-3): `tool_ids`, `okf_namespace_ids` and
  `attachment_ids` (UUIDs), `okf_blob_sha256` (64 lowercase hex) and
  `model`, validated on create and edit (400). Only an agent or bot, or a
  post made on behalf of a person, may set or remove it (403).
- `stream_status` WebSocket event (P-CHIT-4), sent with
  `POST /channels/{id}/stream_status` (`started`, `progress`, `finished`,
  optional `stream_id`) by a member or for one. It goes to the channel's
  members and is never stored; two a second per sender and channel, bursts
  of five, then 429.
- `chit-claude`: control words in a thread (`!new [message]`, `!stop`,
  `!status`); a `⏳ Working on it…` note for runs past 10 seconds, deleted when
  the reply lands; author tags on prompts (`[@alice] …`); a new session
  started partway through a thread is given the thread so far
  (`CHIT_CLAUDE_SEED_MAX_CHARS`, default 8000);
  `CHIT_CLAUDE_MAX_CONCURRENT_RUNS` (default 2);
  `CHIT_CLAUDE_SHUTDOWN_GRACE` (default 60s); `CHIT_CLAUDE_MAX_SESSION_TOKENS`
  to retire a session past a size; `CHIT_CLAUDE_FOOTER` to drop the usage
  line; a `run_complete` log line per run with its token figures.
- `scripts/bridge-token-baseline.sh` compares the tokens a conversation costs
  through the bridge with plain `claude -p`. Measured with `claude` 2.1.292 on
  haiku, the two are within 1% on input.
- `GET /users/me/threads/direct`: the thread inbox for direct and group
  channels, whose threads never appeared in any team's inbox because they
  have no team. `PUT /users/me/threads/{id}/read` and `/following` work
  without a `team_id` for them (the team-scoped routes ignore it anyway).
- OAuth2 authentication for agents and bots through Ory Hydra: Hydra is
  deployed, Oathkeeper introspects `client_credentials` tokens (audience
  `chit`; `chit:read` for GET, `chit:write` otherwise) and forwards the client
  ID as `X-Client-Id`, which chitd resolves against the new
  `users.oauth_client_id` column (migration `000005`). Unknown clients are
  refused, never provisioned. `chit-mcp` and `chit-claude` gain an OAuth mode
  (`*_OAUTH_CLIENT_ID`, `*_OAUTH_CLIENT_SECRET`, `*_OAUTH_TOKEN_URL`) and
  refuse to start without a complete configuration for one of the two
  modes.
- Machine actors: `CHIT_MACHINE_ACTORS` declares agents and bots, which chitd
  creates and reconciles (username, display name, roles, actor type) at boot,
  failing startup on error. Migration `000006` makes `users.kratos_id` and
  `users.email` nullable so such an actor needs no Kratos identity.
- `chit-claude` personas: `CHIT_CLAUDE_MODEL` per bridge, and agent-to-agent
  replies gated by `CHIT_CLAUDE_REPLY_TO_AGENTS` and
  `CHIT_CLAUDE_MAX_AGENT_HOPS`.
- Slash command handlers for `/invite`, `/kick` and `/topic` (`/help`
  already existed). `/summarize` is registered but still has no handler.
- `POST /tags/posts`: tags for up to 200 posts in one request, keyed by post
  ID; posts the caller cannot read are omitted.
- Post search `from` filter (author username, leading `@` accepted).
- The thread inbox returns each thread's root post, `last_viewed_at` and
  `unread_mentions`, and lists only threads in channels the caller still
  belongs to.
- `thread_updated` WebSocket event, sent to the channel after every reply and
  reply deletion with the thread's counters.
- Migration `000007`: indexes on `posts (update_at, id)` and
  `threads (last_reply_at DESC)`, and a partial unique index on
  `channels (name)` for team-less channels, so a DM or group exists once.
- `make vuln` (govulncheck) and `make test-integration` (throwaway
  `postgres:17` in Podman, every migration up/down/up, then the sqlstore and
  pubsub integration suites).
- `chit-tui`: `/group`, `/nick`, `/username`, `/threads` (thread inbox),
  `/leave`, search across every channel (`??`), and a resync when the server
  drops events.
- `chit-tui` terminal client imported into the monorepo at `clients/chit-tui`
  as a nested Go module (own `go.mod`, own dependency tree) with a committed
  root `go.work` workspace. Server builds and images exclude it. New Makefile
  targets: `build-tui`, `test-tui`, `lint-tui`, `test-e2e-tui`, `test-all`.
  TUI docs merged into the main docs site under a "TUI Client" section.
- Global post search endpoint `POST /api/v1/posts/search` — searches across
  every channel the caller is a member of (no team/channel scope)
- `internal/chitclient`: shared HTTP/WebSocket client of chitd used by the
  headless agent binaries (extracted from `internal/bridge` and extended
  with the full REST surface the MCP server needs)
- `make check-deps` dependency-boundary guards (run in CI): client binaries
  must never link server-side deps, chitd must not link the MCP SDK, and
  TUI/charmbracelet deps must stay out of the server module

- `chit-claude` bridge (`cmd/chit-claude`, `internal/bridge`): drives headless
  Claude Code sessions from Chit channels. Thread = session (resumed via
  `claude -p --resume`, session ID stored in reply post props), per-thread run
  serialization, token/cost/context footer from the run's JSON output, and
  error replies on failed runs. Connects to chitd over REST/WebSocket as the
  agent user — no database access. See `docs/.../deployment/headless-claude.mdx`.

- Slash commands framework: CUE-defined commands and roles, Keto-backed
  authorization (`chit/command` namespace, `execute` relation), audit
  logging, and a CloudEvents webhook dispatcher; `chit-reconcile` binary
  pushes CUE definitions into Keto
- Containerized Ory stack (Kratos, Oathkeeper, Keto) in the Podman Kube
  manifests, plus a full UAT environment with pre-seeded users
- `actor_type` column on users (`user` | `agent` | `bot`) — migration
  `000004_actor_type`; humans, AI agents, and bots are equal first-class
  actors
- MCP agent event feed: chit-mcp buffers real-time events for agents to
  poll via the `get_new_events` tool
- Background search indexer: polls posts by an `(update_at, id)` cursor
  every 5 seconds and upserts documents into ZincSearch (keyed by post ID);
  soft-deleted posts are removed from the index; SQL fallback when ZincSearch
  fails. Runs only when `CHIT_ZINCSEARCH_URL` is set.
- GitHub Actions CI workflow: build + unit tests with race detector,
  golangci-lint, govulncheck, integration tests against a PostgreSQL service
  container (after a migration up/down/up round trip), OpenAPI + CUE
  validation
- `make kube-migrate` target to re-apply Chit schema migrations against the
  running pod (kube-up runs all migrations automatically)
- Layered architecture scaffold: API → App → Store → PostgreSQL
- PostgreSQL schema with UUIDv7 primary keys and bigint millisecond timestamps
  (10 tables: users, teams, team_members, channels, channel_members, posts,
  threads, thread_memberships, tags, message_tags)
- OpenAPI 3.1 specification with 40+ endpoints across 10 resource tags
- Podman Kube deployment manifest with 5 containers (PostgreSQL, Kratos,
  Oathkeeper, Keto, ZincSearch)
- Ory stack configuration files (Kratos identity schemas, Oathkeeper access
  rules, Keto namespace configuration)
- WebSocket hub with per-user connection map and single event loop goroutine
- Pub/sub abstraction with PG LISTEN/NOTIFY backend (NATS optional)
- LRU user cache (Kratos ID or OAuth client → user) using
  hashicorp/golang-lru/v2
- MCP server for AI agent integration
- Containerfile for building the server image
- Makefile with build, test, lint, migrate, and kube targets
- Environment-based configuration via koanf with CHIT_ prefix (see
  `.env.example` for the full list)
- Documentation site built with Astro + Starlight
- README, ROADMAP, and CHANGELOG

### Changed

- A deactivated user's Kratos identity is refused with `403` before
  Kratos is asked. chitd used to fetch the identity and fail to save a
  second user for it (`409`).
- `chit-claude` stops gracefully: runs in progress get
  `CHIT_CLAUDE_SHUTDOWN_GRACE` to finish and post their answers, the rest are
  stopped with a reply saying why, and threads with unstarted messages are
  told to resend them. It used to kill every run at once and drop the queue
  without a word.
- `chit-claude`'s footer shows cache reads and writes apart, and measures
  `context ~N%` from the run's last API call against the window the CLI
  reports for the model.
- chitd and `chit-claude` share one mention parser, `model.ParseMentions`.
- `chit-mcp` and `chit-claude` refuse to start when both an OAuth2 client
  (`*_OAUTH_CLIENT_ID`) and a Kratos identity (`*_AGENT_KRATOS_ID`) are set.
  OAuth2 used to win silently. Remove the one you do not mean.
- Anyone may join an open team themselves (`POST /teams/{id}/members` with
  their own `user_id`). Adding someone else, or joining an invite-only team,
  still requires membership. The user added must exist (`400` otherwise).
- Lint is a hard CI gate: `only-new-issues` is gone, lint covers the
  `integration` and `e2e` build tags, staticcheck runs every analyzer but
  ST1000 and ST1003, and golangci-lint is pinned to v2.13.2. The 199
  findings it had been hiding were fixed.
- Go toolchain 1.25.14; chi v5.3.0, pgx v5.9.2, kin-openapi v0.144.0, MCP
  go-sdk v1.4.1, x/net v0.55.0, x/text v0.39.0. govulncheck reports no
  reachable vulnerabilities.
- chi's `RealIP` middleware is removed (spoofable); the request log records
  `X-Forwarded-For` separately as `forwarded_for`.
- Search scope, author and tags are applied before pagination, in one SQL
  query, so pages are full until the last. ZincSearch hits are filtered in
  batches of 100, up to 1,000 per search. A Zinc failure, not an empty
  result, triggers the SQL fallback.
- `PUT /posts/{id}` merges `props` (`null` deletes a key), validates, and
  returns the whole post; `post_edited` carries the whole post.
- `PUT /channels/{id}` and `PUT /teams/{id}` reject a `name` change and can
  clear `header`/`purpose`/`description`.
- User search matches username and display name only, never email.
- Command webhook CloudEvent `id` is the audit log's `event_id`.
- List endpoints return `[]` instead of `null` when empty.
- `chit-reconcile` no longer needs `CHIT_DATABASE_URL`.
- `make run` reads `.env` when it exists.
- The MCP `summarize_channel` prompt rejects a `num_posts` that is not a
  positive integer instead of using 50.
- `chit-mcp` is now an HTTP/WebSocket client of chitd (like `chit-claude`)
  instead of embedding the app/store/PostgreSQL stack in-process. Configure
  with `CHIT_MCP_SERVER_URL`, `CHIT_MCP_AGENT_KRATOS_ID`, and optional
  `CHIT_MCP_PROXY_SECRET`; `CHIT_MCP_AGENT_USER_ID` and database access are
  gone. The agent event feed now consumes chitd's WebSocket: events carry
  full payloads (`posted` events include the entire post) and membership
  filtering happens server-side in the hub. The `mark_thread_read` and
  `follow_thread` MCP tools now take a `team_id` argument.

### Security

- Vulnerable modules updated, clearing the 9 Critical and 24 fixable High
  grype findings in a chitd image: `golang.org/x/crypto` v0.58.0,
  `golang.org/x/net` v0.62.0, `github.com/getkin/kin-openapi` v0.149.0 and
  `github.com/klauspost/compress` v1.20.2 (with `x/sys`, `x/sync` and
  `x/text` as they require). chit-tui takes the same `x/net` and
  `github.com/yuin/goldmark` v1.7.17. The fixed `x/` modules need Go 1.26, so
  both modules and the workspace now require **Go 1.26.10**, and the
  builder images move to it.
- Channel and post authorization enforced server-side: channel membership
  required to read/post/pin, author-or-admin required to edit/delete, team
  membership required to create/join channels, invite-by-member for
  private channels (direct and group membership is now fixed)
- WebSocket events delivered only to channel members (hub membership cache)
- `CHIT_ALLOWED_ORIGINS` origin allowlist for CORS/WebSocket (wildcard logs
  a warning) and `CHIT_TRUSTED_PROXY_SECRET` to gate trust in the
  `X-User-Id` header behind an Oathkeeper-injected `X-Proxy-Secret` header
- Pagination clamped server-side (`page >= 0`, `per_page` between 1 and 200,
  `page * per_page` at most 100,000; a huge page used to overflow into a 500)
- `POST /users` requires `system_admin`; any user could create an admin or
  bind an OAuth client. Adding and removing team members require team
  membership, and removing someone else requires `system_admin`.
- `PUT`/`DELETE /teams/{id}` require `team_admin` or `system_admin`; any
  user could rename or delete any team. Re-adding a team member no longer
  overwrites their roles.
- `GET /teams`, `GET /teams/{id}` and `GET /teams/{id}/members` hide
  invite-only teams and rosters from non-members.
- DM squatting closed: `POST /channels` refused `D`/`G`, DM and group
  membership is fixed, and a member added to an open or private channel must
  be on its team.
- `GET /channels/{id}` requires channel membership (or team membership for an
  open channel); it exposed private headers and DM participants.
- `GET /teams/{id}/channels` no longer lists private channels.
- Replies must target a root post in the same channel; following or marking a
  thread read requires access to its channel; slash commands require channel
  membership before they run. Each let a user read posts in channels they
  were not in.
- `POST /posts` accepts only `channel_id`, `root_id`, `content` and `props`
  (and drops `props.mentions`), so clients cannot set ids, timestamps, pin
  state or type.
- Removing a user from a team removes them from its channels; deleting a
  channel or team ends access to it (a deleted channel has no members).
- Team search (`POST /teams/{id}/posts/search`) ignored the team and checked
  no membership; it now searches the caller's channels in that team.
- User search no longer answers "does an email contain X".
- Request bodies are capped at 1 MiB (`413`); 5xx responses no longer include
  `detailed_error`, which carried SQL and driver text.
- Keto: a permission check that gets an error status is now an error, not a
  silent denial; tuple deletes escape their query parameters.
- Tag operations on a post require membership of its channel; any user could
  tag or list tags on any post.

### Fixed

- `chit-claude` no longer answers another agent as a human after one failed
  author lookup. Failures were cached for the life of the process, which
  lifted `REPLY_TO_AGENTS` and the hop cap for that agent.
- `chit-claude` passes the prompt on stdin. In argv, a post reading
  `--version` was parsed as that option.
- `chit-claude` replaces a thread's session that can no longer be resumed
  (a changed `WORKDIR` or `~/.claude`) with a new one, rather than failing
  every later reply in the thread.
- `chit-claude` posts the CLI's own error message when a run fails with an
  API error, instead of a log tag from stderr; treats `is_error` results as
  failures rather than "_(empty response)_"; splits replies over 64 KiB
  rather than losing them; says so in the thread when its queue is full;
  answers a post delivered twice once; and frees idle threads' workers.
- A cancelled `claude` run whose child processes still held its output could
  outlive its timeout.
- `chit-reconcile` revokes what the CUE no longer grants. It only ever wrote
  tuples, so a command taken from a role, or a role taken from an actor,
  stayed in force in Keto.
- A user removed from a channel now receives `user_removed` themselves, so
  their client can drop the channel; it went only to the remaining members.
- A malformed (non-UUID) channel or team ID in a path or request body is a
  `404`, not a `500` from a failed UUID cast.
- Keto mirror failures when adding or removing channel members are logged,
  not discarded.
- WebSocket: a slow connection among several for one user could panic the
  hub and kill the process; slow clients (full 256-event buffer) are now
  disconnected after the broadcast. Calls after shutdown no longer block;
  pumps start after registration; membership changes during a user's initial
  load are queued instead of lost; membership loads time out after 10 s; an
  unauthenticated upgrade gets a JSON `401`.
- Threads: replying no longer marks the thread unread for the replier; the
  root's author follows their thread from its first reply; deleting a reply
  lowers `reply_count` and recomputes `last_reply_at`.
- Search indexer: posts tied on `update_at` at a batch boundary, or committed
  late, were skipped forever; one post Zinc refused stopped all indexing. The
  cursor is `(update_at, id)` with a 3-second settle lag, and a post that
  fails three times is logged and skipped.
- Duplicate direct channels from concurrent requests (now a unique index, with
  the loser returning the winner's channel).
- `PUT /users/me` mutated the cached user, so a rejected change stayed visible;
  it now validates, lowercases the username, and returns `409` on a taken
  one.
- A machine actor declared without roles lost them on every second boot;
  username and actor type changes were never applied.
- Startup: a listener failure now stops the scheduler, hub, pubsub and store.
- Rate limiter: one goroutine leaked per limiter and every bucket was reset
  every ten minutes; idle buckets are now evicted on access.
- Creating an open channel did per-member work in the request; members
  are added in one statement and Keto tuples are written in the background.
- Mentions cost two queries per name; they are now resolved in one query (at
  most 50 names per post), with counters updated in one statement per table.
  Tags for a page of posts are read in one query.
- Non-UUID user or post IDs return `404` instead of `500`.
- Command authorization never worked: role grants are now written to Keto as
  subject sets.
- `POST /tags/posts` (formerly `/posts/tags`) was unreachable.
- Thread reply counts
- `since`-based post polling on `GET /channels/{id}/posts`
- Store context propagation

### Removed

- Vault Transit Engine integration and the abandoned encryption-at-rest
  column (migration `000003_drop_content_encrypted`)
- Committed `seed-uat` binary removed from git; the seeder is built from
  `scripts/seed-uat/main.go`
- The `status_change` WebSocket event constant, which was never sent.
- Unused `before`/`after` post cursors, which no query read.
