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

### Run again on the same volume

A run does not need a new `chit-pgdata` volume. Scenario 2 uses `test-team`,
`general` and `secret-ops` again when they exist. Scenarios 10 to 12 need
people and channels that are new, so each run adds a random `RUN_ID` to
those names. For example, Erin is `erin-<RUN_ID>` and the HALO team is
`halo-e2e-<RUN_ID>`.

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
  - Setup: The script checks the health of all 6 services (PG, Kratos, Oathkeeper, Keto, ZincSearch, chitd).
    Scenario 11 checks SeaweedFS.
    The script also waits up to 3 minutes for the membership hook stub, because the stub compiles first.
    Then it creates 3 test users through the Kratos Admin API.
    It logs in each user through the API flow to get a session token.
  - Scenario 1: User provisioning — system ping and config at chitd, and 401 through the gateway.
    Automatic provisioning on the first /users/me, user lookup by ID and username, user search.
    Unauthenticated 401, idempotent provisioning.
  - Scenario 2: Team & channel lifecycle — create a team (the creator gets the team_admin role), add members, public and private channels.
    Keto relation tuples (write and delete), the "my channels" filter, updates.
  - Scenario 3: Channel posts — create posts, pagination, get, edit, pin, unpin and delete posts, mark a channel as viewed.
  - Scenario 4: DM & group channels — create a DM (type "D", null team_id, 2 members), Keto DM relations.
    Group channel (type "G", 3 members), and users that read the posts of other users.
  - Scenario 5: Threads — replies with root_id, get a thread, followed threads, mark read, unfollow and follow again, a thread of a standalone post.
  - Scenario 6: Tags & search — create and list tags, tag and untag posts, team and channel search.
    The script still indexes two posts into ZincSearch by hand.
    This step is older than the indexer of chitd.
    When `CHIT_ZINCSEARCH_URL` has a value, the indexer of chitd now syncs posts itself every 5 seconds.
  - Scenario 7: @mentions
  - Scenario 8: Slash commands
  - Scenario 9: Authorization checks
  - Scenario 10: On behalf of a Kratos identity (HALO P-CHIT-8) — the script creates the
    Hydra client `chit-e2e-halod` (`deploy/chit.kube.yml` declares it for chitd).
    It gets a `client_credentials` token.
    Through Oathkeeper, it reads the user of halod.
    It sends a malformed, an unknown and a session `X-On-Behalf-Of`, and chitd refuses each.
    It provisions a Kratos identity that chitd does not know, with `kratos:<id>`.
    As her, it creates her private channel, adds halod, and posts.
    It reads as halod.
    It checks that her own first login finds the same user.
    Then it provisions an identity of the `email-only` Kratos schema (like the schema of HALO).
    It checks the username that chitd makes from that identity.
  - Scenario 11: Attachments (HALO P-CHIT-5) — the script checks that the S3
    gateway of SeaweedFS refuses an anonymous request.
    As halod on behalf of Erin, it uploads a PNG to her Conversation.
    It checks the uploader, `via_actor_id`, and the nosniff and sandbox headers.
    Before the post contains the file, halod gets the file on behalf of Erin.
    chitd refuses the same request from halod as itself.
    It posts the file as Erin and reads it as a member.
    It checks that Bob, who is not a member, gets 403.
    It finds the file by its alt text.
    chitd refuses an SVG, a PNG/HTML polyglot, and a file whose declared type and sniffed type are different.
    chitd refuses an on-behalf-of upload and download from the session of a person.
    chitd also refuses them from `chit-e2e-bot`, a machine actor that is not on `CHIT_ON_BEHALF_OF_CLIENTS`.
    In PostgreSQL, it checks that the same bytes share one object in one channel, and get a different object in another channel.
    It checks that when it deletes the post, chitd deletes the file.
  - Scenario 12: Membership policy hook (HALO P-CHIT-2) — before each channel add, chitd asks the
    `memberhook-stub` container of `deploy/chit.kube.yml`.
    The stub is `tests/e2e/hookstub`, a stand-in for halod on 127.0.0.1:4490.
    chitd uses the Hydra client `chit-e2e-memberhook`.
    The stub registers this client and checks its tokens through the Hydra introspect endpoint.
    The scenario checks these items:
    the stub refuses a token that Hydra did not issue to chitd.
    For the on-behalf-of add of halod in scenario 10, chitd asked about halod (an agent, added by Erin, via halod).
    chitd did not ask about Erin, the creator.
    The request body of an allowed add.
    A refused user (Charlie) gets 403 `channel.membership_refused` with the `reason` and `detail` of the hook.
    This occurs on `POST /channels/{id}/members` and on `/invite` (with a temporary Keto grant).
    It also occurs when a user creates an Open channel in a team that has Charlie (chitd does not create the channel).
    It also occurs when Charlie joins `general`, and for a DM and a group with Charlie.
    When the hook fails, chitd refuses with `hook_unavailable`.
    The audit log records the answers.
    `HOOK_STUB` and `E2E_AUDIT` point the scenario at a different stub or pod.
  - Scenario 13: Rate limits (HALO P-CHIT-8) — the script sends 120 requests at the same time, 16
    in parallel.
    As halod itself, chitd serves all of them (the aggregate limit of halod applies, not the per-user burst of 50).
    On behalf of Erin, chitd refuses some with 429 and `Retry-After: 1` (her own per-user budget).
    At the same time, chitd still serves halod as itself and Alice.
    As `chit-e2e-bot`, a machine actor that is not on `CHIT_ON_BEHALF_OF_CLIENTS`, chitd refuses some (the per-user limit).
    Then the script checks the log of chitd for the `rate limit exceeded` lines: the bucket, the
    actor and client ID, and Erin as the principal.
    `E2E_CHITD_LOG` points the scenario at a different pod.

  The Keto checks in scenarios 2 and 4 read `chit/channel` tuples. chitd
  writes these tuples best-effort. For the members of a new channel, chitd
  writes the tuples in the background. So those tuples can appear a moment
  after the response.

  2. deploy/oathkeeper/oathkeeper.yml

  Humans authenticate with the `bearer_token` authenticator. Oathkeeper reads
  the Kratos session token from `X-Session-Token`. It forwards the token to
  `/sessions/whoami` (`forward_http_headers: ["X-Session-Token"]`). So Kratos
  API-flow session tokens work, not only browser cookies.
