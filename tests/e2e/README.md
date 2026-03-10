# E2E Tests

## How to run

```bash
  # 1. Kill the stale chitd process (kill ALL chitd, not just shell job %1)                                                                                                 
  pkill -f chitd 2>/dev/null                                                                                                                                                
  sleep 1                                                                                                                                                                                                                                                                                                           
  # 2. Rebuild the binary with the latest code
  make build

  # 3. Restart services
  make kube-down && make kube-up
  sleep 10

  # 4. Run migrations
  make kube-migrate
  ~/go/bin/migrate -path migrations -database "postgres://chit:chit@localhost:5432/chit?sslmode=disable" up

  # 5. Start the NEW binary
  CHIT_DATABASE_URL="postgres://chit:chit@localhost:5432/chit?sslmode=disable" ./bin/chitd &
  sleep 2

  # 6. Run tests
  bash tests/e2e/test_e2e.sh
```

## Details

Created/Modified Files                                                                                                                                                                  
                                                                                                                    
  1. tests/e2e/test_e2e.sh (new, 898 lines)                                                                                                                                         
                                                                                                                                                                                        
  Executable bash script with:                                                                                                                                                            
  - Setup: Health checks for all 7 services (PG, Kratos, Oathkeeper, Keto, Vault, ZincSearch, chitd), creates 3 test users via Kratos Admin API, logs in each via API flow to get session 
  tokens, enables Vault transit key                                                                                                                                                       
  - Scenario 1 (11 assertions): User provisioning — system ping/config, auto-provision on first /users/me, user lookup by ID/username, user search, unauthenticated 401, idempotent       
  provisioning                                                                                                                                                                            
  - Scenario 2 (19 assertions): Team & channel lifecycle — team creation with auto team_admin role, adding members, public/private channels, Keto relation tuple verification (write &    
  delete), "my channels" filtering, updates
  - Scenario 3 (13 assertions): Channel messaging — posting, pagination, get/edit/pin/unpin/delete posts, mark channel viewed
  - Scenario 4 (11 assertions): DM & group channels — DM creation (type "D", null team_id, 2 members), Keto DM relations, group channel (type "G", 3 members), cross-user message reading
  - Scenario 5 (10 assertions): Threading — replies with root_id, thread retrieval, followed threads, mark read, unfollow/re-follow, standalone post thread
  - Scenario 6 (10 assertions): Tags & search — create/list tags, tag/untag posts, manual ZincSearch indexing (workaround for skeleton indexer), team/channel search

  2. deploy/oathkeeper/oathkeeper.yml (modified)

  Added forward_http_headers: ["X-Session-Token"] to the cookie_session authenticator config so Oathkeeper forwards Kratos API-flow session tokens to /sessions/whoami for validation.
  Without this, only browser cookies would work.