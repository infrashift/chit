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
(`podman stop chit-app-chitd`) and start your build on `:8065`, e.g.
`cp .env.example .env && make run`. Oathkeeper forwards to whatever listens
there.

### Unauthenticated checks

Through the gateway every `/api/v1` route needs a session or token, ping
included; `/system/ping` and `/system/config/client` are public only at chitd
itself (`:8065`). The script checks them there (assertions 1.1 and 1.2), and
checks that the gateway refuses the same unauthenticated ping with 401 (the
setup health check, and 1.1b).

## Details

Created/Modified Files

  1. tests/e2e/test_e2e.sh

  Executable bash script with:
  - Setup: Health checks for all 6 services (PG, Kratos, Oathkeeper, Keto, ZincSearch, chitd), creates 3 test users via Kratos Admin API, logs in each via API flow to get session tokens
  - Scenario 1: User provisioning — system ping/config at chitd and 401 through the gateway, auto-provision on first /users/me, user lookup by ID/username, user search, unauthenticated 401, idempotent provisioning
  - Scenario 2: Team & channel lifecycle — team creation with auto team_admin role, adding members, public/private channels, Keto relation tuple verification (write & delete), "my channels" filtering, updates
  - Scenario 3: Channel messaging — posting, pagination, get/edit/pin/unpin/delete posts, mark channel viewed
  - Scenario 4: DM & group channels — DM creation (type "D", null team_id, 2 members), Keto DM relations, group channel (type "G", 3 members), cross-user message reading
  - Scenario 5: Threading — replies with root_id, thread retrieval, followed threads, mark read, unfollow/re-follow, standalone post thread
  - Scenario 6: Tags & search — create/list tags, tag/untag posts, team/channel search. The script still indexes two posts into ZincSearch by hand; that predates chitd's indexer, which now syncs posts itself every 5 seconds when `CHIT_ZINCSEARCH_URL` is set.
  - Scenario 7: @mentions
  - Scenario 8: Slash commands
  - Scenario 9: Authorization enforcement

  The Keto checks in scenarios 2 and 4 read `chit/channel` tuples, which chitd
  writes best-effort; members added when a channel is created are written in
  the background, so those tuples can appear a moment after the response.

  2. deploy/oathkeeper/oathkeeper.yml

  Humans authenticate with the `bearer_token` authenticator: Oathkeeper reads
  the Kratos session token from `X-Session-Token` and forwards it to
  `/sessions/whoami` (`forward_http_headers: ["X-Session-Token"]`), so
  Kratos API-flow session tokens work, not only browser cookies.
