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
  - Setup: Health checks for all 6 services (PG, Kratos, Oathkeeper, Keto, ZincSearch, chitd). Then the script creates 3 test users via the Kratos Admin API. It logs in each user via the API flow to get session tokens
  - Scenario 1: User provisioning — system ping/config at chitd and 401 through the gateway. Then auto-provisioning on the first /users/me, user lookup by ID/username, user search, unauthenticated 401, idempotent provisioning
  - Scenario 2: Team & channel lifecycle — a new team gives its creator the team_admin role. Then new members, public/private channels, Keto relation tuples (write & delete), the "my channels" filter, updates
  - Scenario 3: Channel posts — new posts, pagination, get/edit/pin/unpin/delete posts, mark channel viewed
  - Scenario 4: DM & group channels — a new DM (type "D", null team_id, 2 members) and its Keto relations. Then a group channel (type "G", 3 members), and one user reads the messages of another
  - Scenario 5: Threads — replies with root_id, thread retrieval, followed threads, mark read, unfollow/re-follow, standalone post thread
  - Scenario 6: Tags & search — create/list tags, tag/untag posts, team/channel search. The script still indexes two posts into ZincSearch by hand. That step is older than the chitd indexer. When the operator sets `CHIT_ZINCSEARCH_URL`, the indexer now syncs posts itself every 5 seconds.
  - Scenario 7: @mentions
  - Scenario 8: Slash commands
  - Scenario 9: Authorization checks
  - Scenario 10: On behalf of a Kratos identity (HALO P-CHIT-8). The script
    creates the Hydra client `chit-e2e-halod` (declared for chitd in
    `deploy/chit.kube.yml`) and takes a `client_credentials` token. Then,
    through Oathkeeper, it does these steps. It reads halod's own user. It
    sends a malformed, an unknown and a session `X-On-Behalf-Of`, and checks
    that chitd refuses each one. It provisions a never-seen Kratos identity
    with `kratos:<id>`. It creates her private channel and adds halod as her.
    It posts as her and reads as halod. It checks that her own first login
    finds the same user. Last, it provisions an identity of the `email-only`
    Kratos schema (like HALO's) and checks its derived username

  The Keto checks in scenarios 2 and 4 read `chit/channel` tuples. chitd
  writes these tuples best-effort. For the members of a new channel, chitd
  writes the tuples in the background. So those tuples can appear a moment
  after the response.

  2. deploy/oathkeeper/oathkeeper.yml

  Humans authenticate with the `bearer_token` authenticator. Oathkeeper reads
  the Kratos session token from `X-Session-Token`. It forwards the token to
  `/sessions/whoami` (`forward_http_headers: ["X-Session-Token"]`). So Kratos
  API-flow session tokens work, not only browser cookies.
