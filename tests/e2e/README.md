# E2E Tests

`tests/e2e/test_e2e.sh` drives the full stack with curl and jq: Kratos →
Oathkeeper → chitd → PostgreSQL → Keto (and ZincSearch).

This is separate from `make test-e2e`, which runs the integration-tagged
store and pub/sub Go tests in a container against PostgreSQL only.

## How to run

```bash
  # 1. Rebuild the chitd image with the latest code and (re)start the stack.
  #    `make kube-up` runs chitd in the chit-app pod on :8065; Kratos, Keto,
  #    Hydra, and chit migrations run automatically in init containers.
  make kube-down && make kube-up
  sleep 10

  # 2. (Only if new migration files were added since the pod started)
  #    re-apply chit schema migrations against the running pod
  # make kube-migrate

  # 3. Run tests
  bash tests/e2e/test_e2e.sh
```

To test a locally built chitd instead of the image, stop the pod's chitd
(`podman stop chit-app-chitd`) and start the local build on `:8065`, e.g.
`cp .env.example .env && make run`. Oathkeeper forwards to whatever listens
there.

### Unauthenticated checks

Through the gateway every `/api/v1` route needs a session or token, ping
included. `/system/ping` and `/system/config/client` are public only at chitd
itself (`:8065`). The script checks them there (assertions 1.1 and 1.2). It
also checks that the gateway refuses the same unauthenticated ping with 401
(the setup health check, and 1.1b).

## Details

Created/Modified Files

  1. tests/e2e/test_e2e.sh

  Executable bash script with:
  - Setup: Health checks for all 6 services (PG, Kratos, Oathkeeper, Keto, ZincSearch, chitd; SeaweedFS is checked in scenario 11) and the membership hook stub (waited for up to 3 minutes while it compiles), creates 3 test users via Kratos Admin API, logs in each via API flow to get session tokens
  - Scenario 1: User provisioning — system ping/config at chitd and 401 through the gateway, auto-provision on first /users/me, user lookup by ID/username, user search, unauthenticated 401, idempotent provisioning
  - Scenario 2: Team & channel lifecycle — team creation with auto team_admin role, adding members, public/private channels, Keto relation tuple verification (write & delete), "my channels" filtering, updates
  - Scenario 3: Channel messaging — posting, pagination, get/edit/pin/unpin/delete posts, mark channel viewed
  - Scenario 4: DM & group channels — DM creation (type "D", null team_id, 2 members), Keto DM relations, group channel (type "G", 3 members), cross-user message reading
  - Scenario 5: Threading — replies with root_id, thread retrieval, followed threads, mark read, unfollow/re-follow, standalone post thread
  - Scenario 6: Tags & search — create/list tags, tag/untag posts, team/channel search. The script still indexes two posts into ZincSearch by hand; that predates chitd's indexer, which now syncs posts itself every 5 seconds when `CHIT_ZINCSEARCH_URL` is set.
  - Scenario 7: @mentions
  - Scenario 8: Slash commands
  - Scenario 9: Authorization enforcement
  - Scenario 10: On behalf of a Kratos identity (HALO P-CHIT-8) — creates the
    Hydra client `chit-e2e-halod` (declared for chitd in `deploy/chit.kube.yml`),
    takes a `client_credentials` token, and through Oathkeeper: reads halod's
    own user, refuses a malformed, unknown and session `X-On-Behalf-Of`,
    provisions a never-seen Kratos identity with `kratos:<id>`, creates her
    private channel and adds halod as her, posts as her, reads as halod, and
    checks that her own first login finds the same user; then provisions an
    identity of the `email-only` Kratos schema (like HALO's) and checks its
    derived username
  - Scenario 11: Attachments (HALO P-CHIT-5) — checks that SeaweedFS's S3
    gateway refuses an anonymous request; as halod on behalf of Erin, uploads
    a PNG to her Conversation (uploader and `via_actor_id`, nosniff and
    sandbox headers), fetches it as her before it is attached and is refused
    as itself, posts it as her and reads it as a member, and checks that Bob,
    not a member, gets 403; finds it by alt text; refuses SVG, a PNG/HTML
    polyglot and a declared/sniffed mismatch; refuses on-behalf-of upload and
    download from a person's session and from `chit-e2e-bot`, a machine actor
    not on `CHIT_ON_BEHALF_OF_CLIENTS`; checks in PostgreSQL that the same
    bytes share one object within a channel and get their own in another;
    and that deleting the post deletes the file
  - Scenario 12: Membership policy hook (HALO P-CHIT-2) — chitd asks the
    `memberhook-stub` container of `deploy/chit.kube.yml`
    (`tests/e2e/hookstub`, a stand-in for halod on 127.0.0.1:4490) before
    every channel add, with the Hydra client `chit-e2e-memberhook` that the
    stub registers and checks by introspection. Checks that the stub refuses
    a token Hydra did not issue to chitd; that scenario 10's on-behalf-of add
    of halod asked about halod (an agent, added by Erin, via halod) and not
    about Erin, the creator; the request body of an allowed add; that a
    refused user (Charlie) gets 403 `channel.membership_refused` with the
    hook's `reason` and `detail` on `POST /channels/{id}/members`, `/invite`
    (with a temporary Keto grant), an Open channel's creation (no channel is
    created), joining `general`, a DM and a group; that a failing hook
    refuses with `hook_unavailable`; and that the audit log records the
    answers. `HOOK_STUB` and `E2E_AUDIT` point the scenario at another stub
    or pod.
  - Scenario 13: Rate limits (HALO P-CHIT-8) — sends 120 requests at once, 16
    in parallel: as halod itself, all are served (its aggregate limit, not
    the per-user burst of 50); on behalf of Erin, some are refused with 429
    and `Retry-After: 1` (her own per-user budget), while halod as itself
    and Alice are still served; as `chit-e2e-bot`, a machine actor not on
    `CHIT_ON_BEHALF_OF_CLIENTS`, some are refused (the per-user limit). Then
    checks chitd's log for the `rate limit exceeded` lines: the bucket, the
    actor and client ID, and Erin as the principal. `E2E_CHITD_LOG` points
    it at another pod.

  The Keto checks in scenarios 2 and 4 read `chit/channel` tuples. chitd
  writes these tuples best-effort. For the members of a new channel, chitd
  writes the tuples in the background. So those tuples can appear a moment
  after the response.

  2. deploy/oathkeeper/oathkeeper.yml

  Humans authenticate with the `bearer_token` authenticator. Oathkeeper reads
  the Kratos session token from `X-Session-Token`. It forwards the token to
  `/sessions/whoami` (`forward_http_headers: ["X-Session-Token"]`). So Kratos
  API-flow session tokens work, not only browser cookies.
