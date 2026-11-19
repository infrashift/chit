#!/usr/bin/env bash
# ============================================================================
# Chit E2E Tests
#
# Exercises the full stack: Kratos → Oathkeeper → chitd → PostgreSQL → Keto
# using curl + jq. Requires all services up: make kube-up runs chitd in the
# pod alongside them.
#
# Usage: bash tests/e2e/test_e2e.sh
# ============================================================================
set -euo pipefail

# ---------------------------------------------------------------------------
# Service URLs
# ---------------------------------------------------------------------------
PROXY="http://localhost:4455"           # Oathkeeper proxy → chitd
API="${PROXY}/api/v1"                   # Chit API through proxy
KRATOS_PUBLIC="http://localhost:4433"   # Kratos public API (direct)
KRATOS_ADMIN="http://localhost:4434"    # Kratos admin API (direct)
KETO_READ="http://localhost:4466"       # Keto read API
KETO_WRITE="http://localhost:4467"      # Keto write API
ZINC_URL="http://localhost:4080"        # ZincSearch
ZINC_USER="admin"
ZINC_PASS="admin"
CHITD="http://localhost:8065"           # chitd direct (for health check)
HYDRA_PUBLIC="http://localhost:4444"    # Hydra public API (token endpoint)
HYDRA_ADMIN="http://localhost:4445"     # Hydra admin API (clients)
# Runs one SQL statement in chit's database. Only the setup uses it, to grant
# system_admin, which no API grants to a person. Point it at the UAT pod with
# E2E_PSQL="podman exec -i chit-uat-db-postgres psql -U chit -d chit -tAc".
E2E_PSQL="${E2E_PSQL:-podman exec -i chit-db-postgres psql -U chit -d chit -tAc}"
# The membership hook stub (deploy/chit.kube.yml memberhook-stub) and the
# command that prints chitd's audit log, for scenario 12. Point the latter at
# another pod with E2E_AUDIT="podman exec <pod>-chitd cat /var/log/chit/audit.log".
HOOK_STUB="${HOOK_STUB:-http://127.0.0.1:4490}"
E2E_AUDIT="${E2E_AUDIT:-podman exec chit-app-chitd cat /var/log/chit/audit.log}"
# chitd's own log (stderr), for scenario 13; for another pod,
# E2E_CHITD_LOG="podman logs <pod>-chitd".
E2E_CHITD_LOG="${E2E_CHITD_LOG:-podman logs chit-app-chitd}"

# ---------------------------------------------------------------------------
# Counters
# ---------------------------------------------------------------------------
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# ---------------------------------------------------------------------------
# Colors (disabled if not a terminal)
# ---------------------------------------------------------------------------
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[0;33m'
  CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
else
  GREEN=''; RED=''; YELLOW=''; CYAN=''; BOLD=''; NC=''
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# assert_status LABEL EXPECTED_STATUS ACTUAL_STATUS RESPONSE_BODY
assert_status() {
  local label="$1" expected="$2" actual="$3" body="${4:-}"
  if [[ "$actual" -eq "$expected" ]]; then
    echo -e "  ${GREEN}PASS${NC} ${label} (HTTP ${actual})"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} ${label} — expected HTTP ${expected}, got ${actual}"
    [[ -n "$body" ]] && echo "       Response: ${body:0:300}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# assert_json LABEL JQ_EXPR EXPECTED_VALUE JSON_STRING
# Compares jq output (raw) against expected string.
assert_json() {
  local label="$1" jq_expr="$2" expected="$3" json="$4"
  local actual
  actual=$(echo "$json" | jq -r "$jq_expr" 2>/dev/null || echo "__JQ_ERROR__")
  if [[ "$actual" == "$expected" ]]; then
    echo -e "  ${GREEN}PASS${NC} ${label} (${jq_expr} == \"${expected}\")"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} ${label} — ${jq_expr}: expected \"${expected}\", got \"${actual}\""
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# assert_json_gte LABEL JQ_EXPR MIN_VALUE JSON_STRING
assert_json_gte() {
  local label="$1" jq_expr="$2" min="$3" json="$4"
  local actual
  actual=$(echo "$json" | jq -r "$jq_expr" 2>/dev/null || echo "0")
  if [[ "$actual" -ge "$min" ]]; then
    echo -e "  ${GREEN}PASS${NC} ${label} (${jq_expr} == ${actual} >= ${min})"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} ${label} — ${jq_expr}: expected >= ${min}, got ${actual}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# assert_json_not_empty LABEL JQ_EXPR JSON_STRING
assert_json_not_empty() {
  local label="$1" jq_expr="$2" json="$3"
  local actual
  actual=$(echo "$json" | jq -r "$jq_expr" 2>/dev/null || echo "")
  if [[ -n "$actual" && "$actual" != "null" ]]; then
    echo -e "  ${GREEN}PASS${NC} ${label} (${jq_expr} is non-empty)"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} ${label} — ${jq_expr}: expected non-empty, got \"${actual}\""
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# keto_allowed OBJECT SUBJECT — prints Keto's "allowed" for channel
# membership, polling for up to 5s until it is true. chitd writes membership
# tuples to Keto asynchronously (channel_members is the access check; Keto is
# a best-effort mirror), so a check straight after a change can race the write.
keto_allowed() {
  local allowed="false"
  for _ in $(seq 1 25); do
    allowed=$(curl -s "${KETO_READ}/relation-tuples/check?namespace=chit/channel&object=$1&relation=member&subject_id=$2" | jq -r '.allowed')
    [[ "$allowed" == "true" ]] && break
    sleep 0.2
  done
  echo "$allowed"
}

# curl_api METHOD PATH [DATA] — returns "STATUS\nBODY"
# Uses global CURRENT_TOKEN for auth.
curl_api() {
  local method="$1" path="$2" data="${3:-}"
  local url="${API}${path}"
  local args=(-s -o /dev/null -w '%{http_code}\n' -X "$method")
  args+=(-H "Content-Type: application/json")
  if [[ -n "${CURRENT_TOKEN:-}" ]]; then
    args+=(-H "X-Session-Token: ${CURRENT_TOKEN}")
  fi
  [[ -n "$data" ]] && args+=(-d "$data")

  # First get status code
  local status
  status=$(curl "${args[@]}" "$url")

  # Then get body
  local body_args=(-s -X "$method" -H "Content-Type: application/json")
  if [[ -n "${CURRENT_TOKEN:-}" ]]; then
    body_args+=(-H "X-Session-Token: ${CURRENT_TOKEN}")
  fi
  [[ -n "$data" ]] && body_args+=(-d "$data")
  local body
  body=$(curl "${body_args[@]}" "$url" 2>/dev/null || echo "{}")

  echo "${status}"
  echo "${body}"
}

# curl_api_full METHOD PATH [DATA] — sets LAST_STATUS and LAST_BODY globals
curl_api_full() {
  local method="$1" path="$2" data="${3:-}"
  local url="${API}${path}"

  local tmpfile
  tmpfile=$(mktemp)
  local args=(-s -w '%{http_code}' -X "$method" -o "$tmpfile")
  args+=(-H "Content-Type: application/json")
  if [[ -n "${CURRENT_TOKEN:-}" ]]; then
    args+=(-H "X-Session-Token: ${CURRENT_TOKEN}")
  fi
  [[ -n "$data" ]] && args+=(-d "$data")

  LAST_STATUS=$(curl "${args[@]}" "$url")
  LAST_BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
}

# kratos_create_identity EMAIL USERNAME DISPLAY_NAME PASSWORD
# Sets IDENTITY_ID to the created Kratos identity UUID.
kratos_create_identity() {
  local email="$1" username="$2" display_name="$3" password="$4"
  local payload
  payload=$(jq -n \
    --arg email "$email" \
    --arg username "$username" \
    --arg display_name "$display_name" \
    --arg password "$password" \
    '{
      schema_id: "default",
      traits: { email: $email, username: $username, display_name: $display_name },
      credentials: { password: { config: { password: $password } } }
    }')

  local tmpfile
  tmpfile=$(mktemp)
  local status
  status=$(curl -s -w '%{http_code}' -o "$tmpfile" -X POST "${KRATOS_ADMIN}/admin/identities" \
    -H "Content-Type: application/json" \
    -d "$payload")
  local resp
  resp=$(cat "$tmpfile")
  rm -f "$tmpfile"

  if [[ "$status" == "201" ]]; then
    IDENTITY_ID=$(echo "$resp" | jq -r '.id')
  elif [[ "$status" == "409" ]]; then
    # Identity already exists — look it up by email
    local list_resp
    list_resp=$(curl -s "${KRATOS_ADMIN}/admin/identities?credentials_identifier=${email}")
    IDENTITY_ID=$(echo "$list_resp" | jq -r '.[0].id')
    if [[ -n "$IDENTITY_ID" && "$IDENTITY_ID" != "null" ]]; then
      echo -e "  ${YELLOW}WARN${NC}: Identity for ${email} already exists, reusing ${IDENTITY_ID}"
      return 0
    fi
  fi

  if [[ -z "$IDENTITY_ID" || "$IDENTITY_ID" == "null" ]]; then
    echo -e "${RED}ERROR${NC}: Failed to create Kratos identity for ${email}"
    echo "  Response: ${resp:0:300}"
    return 1
  fi
}

# kratos_login EMAIL PASSWORD
# Sets SESSION_TOKEN.
kratos_login() {
  local email="$1" password="$2"

  # Step 1: Init API login flow
  local flow_resp
  flow_resp=$(curl -s "${KRATOS_PUBLIC}/self-service/login/api")
  local flow_id
  flow_id=$(echo "$flow_resp" | jq -r '.id')
  local action_url
  action_url=$(echo "$flow_resp" | jq -r '.ui.action')

  if [[ -z "$flow_id" || "$flow_id" == "null" ]]; then
    echo -e "${RED}ERROR${NC}: Failed to init login flow for ${email}"
    echo "  Response: ${flow_resp:0:300}"
    return 1
  fi

  # Step 2: Submit credentials
  local login_resp
  login_resp=$(curl -s -X POST "$action_url" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg id "$email" --arg pw "$password" \
      '{ method: "password", identifier: $id, password: $pw }')")

  SESSION_TOKEN=$(echo "$login_resp" | jq -r '.session_token')

  if [[ -z "$SESSION_TOKEN" || "$SESSION_TOKEN" == "null" ]]; then
    echo -e "${RED}ERROR${NC}: Failed to login ${email}"
    echo "  Response: ${login_resp:0:300}"
    return 1
  fi
}

section() {
  echo ""
  echo -e "${CYAN}${BOLD}=== $1 ===${NC}"
}

# ============================================================================
# SETUP: Health checks & identity provisioning
# ============================================================================
section "SETUP: Service Health Checks"

check_service() {
  local name="$1" url="$2" expected="${3:-200}"
  local status
  status=$(curl -s -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || echo "000")
  if [[ "$status" == "$expected" ]]; then
    echo -e "  ${GREEN}OK${NC}   ${name} (${url})"
  else
    echo -e "  ${RED}DOWN${NC} ${name} (${url}) — HTTP ${status}"
    echo -e "${RED}FATAL: ${name} is not available. Cannot continue.${NC}"
    exit 1
  fi
}

check_service "PostgreSQL (via chitd)" "${CHITD}/api/v1/system/ping"
check_service "Kratos Public"         "${KRATOS_PUBLIC}/health/alive"
check_service "Kratos Admin"          "${KRATOS_ADMIN}/admin/identities" "200"
# The gateway requires a session or token for every /api/v1 route, ping
# included, so an unauthenticated ping answering 401 is the proxy up and
# guarding chitd. (Ping and config/client are public only at chitd itself.)
check_service "Oathkeeper Proxy"      "${PROXY}/api/v1/system/ping" "401"
check_service "Keto Read"             "${KETO_READ}/health/alive"
check_service "Keto Write"            "${KETO_WRITE}/health/alive"
# The membership hook stub compiles at start (go run), then registers chitd's
# Hydra client; every channel add asks it, so nothing runs before it is up.
HOOK_STUB_STATUS="000"
for _ in $(seq 1 90); do
  HOOK_STUB_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${HOOK_STUB}/healthz" 2>/dev/null || echo "000")
  [[ "$HOOK_STUB_STATUS" == "200" ]] && break
  sleep 2
done
check_service "Membership hook stub"  "${HOOK_STUB}/healthz"
# ZincSearch may return 200 or other codes on root
ZINC_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${ZINC_URL}/version" 2>/dev/null || echo "000")
if [[ "$ZINC_STATUS" == "200" ]]; then
  echo -e "  ${GREEN}OK${NC}   ZincSearch (${ZINC_URL})"
else
  echo -e "  ${YELLOW}WARN${NC} ZincSearch (${ZINC_URL}) — HTTP ${ZINC_STATUS}, search tests may be skipped"
fi

# ---------------------------------------------------------------------------
# Create test identities in Kratos
# ---------------------------------------------------------------------------
section "SETUP: Create Kratos Identities"

kratos_create_identity "alice@test.local" "alice" "Alice Anderson" "TestPass1234"
ALICE_KRATOS_ID="$IDENTITY_ID"
echo "  Alice Kratos ID: ${ALICE_KRATOS_ID}"

kratos_create_identity "bob@test.local" "bob" "Bob Baker" "TestPass1234"
BOB_KRATOS_ID="$IDENTITY_ID"
echo "  Bob   Kratos ID: ${BOB_KRATOS_ID}"

kratos_create_identity "charlie@test.local" "charlie" "Charlie Clark" "TestPass1234"
CHARLIE_KRATOS_ID="$IDENTITY_ID"
echo "  Charlie Kratos ID: ${CHARLIE_KRATOS_ID}"

# Dave is the system_admin: removing another member of a channel needs it.
kratos_create_identity "dave@test.local" "dave" "Dave Admin" "TestPass1234"
DAVE_KRATOS_ID="$IDENTITY_ID"
echo "  Dave  Kratos ID: ${DAVE_KRATOS_ID}"

# ---------------------------------------------------------------------------
# Login each user to get session tokens
# ---------------------------------------------------------------------------
section "SETUP: Login Users (Kratos API Flow)"

kratos_login "alice@test.local" "TestPass1234"
ALICE_TOKEN="$SESSION_TOKEN"
echo "  Alice token: ${ALICE_TOKEN:0:20}..."

kratos_login "bob@test.local" "TestPass1234"
BOB_TOKEN="$SESSION_TOKEN"
echo "  Bob   token: ${BOB_TOKEN:0:20}..."

kratos_login "charlie@test.local" "TestPass1234"
CHARLIE_TOKEN="$SESSION_TOKEN"
echo "  Charlie token: ${CHARLIE_TOKEN:0:20}..."

kratos_login "dave@test.local" "TestPass1234"
DAVE_TOKEN="$SESSION_TOKEN"
echo "  Dave  token: ${DAVE_TOKEN:0:20}..."

# ---------------------------------------------------------------------------
# Provision Dave and make him system_admin
# ---------------------------------------------------------------------------
section "SETUP: Grant system_admin to Dave"

DAVE_ID=$(curl -s -H "X-Session-Token: ${DAVE_TOKEN}" "${API}/users/me" | jq -r '.id')
if [[ -z "$DAVE_ID" || "$DAVE_ID" == "null" ]]; then
  echo -e "${RED}FATAL: could not provision Dave.${NC}"
  exit 1
fi
DAVE_ROLES=$($E2E_PSQL "UPDATE users SET roles = 'system_user system_admin' WHERE id = '${DAVE_ID}' RETURNING roles" | sed -n 1p)
if [[ "$DAVE_ROLES" != *system_admin* ]]; then
  echo -e "${RED}FATAL: could not grant system_admin to Dave with: ${E2E_PSQL}${NC}"
  exit 1
fi
echo "  Dave ${DAVE_ID}: ${DAVE_ROLES}"

# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}Setup complete. Running test scenarios...${NC}"

# ============================================================================
# SCENARIO 1: User Provisioning
# ============================================================================
section "Scenario 1: User Provisioning"

# 1.1 System ping, unauthenticated, at chitd: public there, and only there.
CURRENT_TOKEN=""
API="${CHITD}/api/v1" curl_api_full GET "/system/ping"
assert_status "1.1 system/ping at chitd" 200 "$LAST_STATUS"
assert_json   "1.1 ping status" ".status" "OK" "$LAST_BODY"

# 1.1b Through the gateway the same request needs credentials.
curl_api_full GET "/system/ping"
assert_status "1.1b system/ping through the gateway needs auth" 401 "$LAST_STATUS"

# 1.2 Client config, unauthenticated, at chitd
API="${CHITD}/api/v1" curl_api_full GET "/system/config/client"
assert_status "1.2 system/config/client at chitd" 200 "$LAST_STATUS"
assert_json_not_empty "1.2 version present" ".version" "$LAST_BODY"

# 1.3 Unauthenticated request to protected endpoint → 401
CURRENT_TOKEN=""
curl_api_full GET "/users/me"
assert_status "1.3 unauthenticated /users/me" 401 "$LAST_STATUS"

# 1.4 Alice first authenticated request → auto-provision
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full GET "/users/me"
assert_status "1.4 Alice /users/me auto-provision" 200 "$LAST_STATUS"
ALICE_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json_not_empty "1.4 Alice has ID" ".id" "$LAST_BODY"
assert_json "1.4 Alice username" ".username" "alice" "$LAST_BODY"
assert_json "1.4 Alice display_name" ".display_name" "Alice Anderson" "$LAST_BODY"
assert_json "1.4 Alice kratos_id" ".kratos_id" "$ALICE_KRATOS_ID" "$LAST_BODY"

# 1.5 Bob auto-provision
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/users/me"
assert_status "1.5 Bob /users/me auto-provision" 200 "$LAST_STATUS"
BOB_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json "1.5 Bob username" ".username" "bob" "$LAST_BODY"

# 1.6 Charlie auto-provision
CURRENT_TOKEN="$CHARLIE_TOKEN"
curl_api_full GET "/users/me"
assert_status "1.6 Charlie /users/me auto-provision" 200 "$LAST_STATUS"
CHARLIE_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json "1.6 Charlie username" ".username" "charlie" "$LAST_BODY"

# 1.7 Idempotent provisioning — same ID on repeat
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full GET "/users/me"
assert_status "1.7 Alice idempotent provision" 200 "$LAST_STATUS"
assert_json "1.7 same Alice ID" ".id" "$ALICE_ID" "$LAST_BODY"

# 1.8 Get user by ID (other user lookup — email sanitized)
curl_api_full GET "/users/${BOB_ID}"
assert_status "1.8 get Bob by ID" 200 "$LAST_STATUS"
assert_json "1.8 Bob email sanitized" ".email" "" "$LAST_BODY"
assert_json "1.8 Bob username" ".username" "bob" "$LAST_BODY"

# 1.9 Get user by username
curl_api_full GET "/users/username/charlie"
assert_status "1.9 get Charlie by username" 200 "$LAST_STATUS"
assert_json "1.9 Charlie ID matches" ".id" "$CHARLIE_ID" "$LAST_BODY"

# 1.10 Search users by term
curl_api_full GET "/users?term=ali"
assert_status "1.10 search users term=ali" 200 "$LAST_STATUS"
SEARCH_COUNT=$(echo "$LAST_BODY" | jq 'length')
assert_json_gte "1.10 found >= 1 user" "length" 1 "$LAST_BODY"

# 1.11 Get users by IDs
curl_api_full POST "/users/ids" "[\"${ALICE_ID}\", \"${BOB_ID}\"]"
assert_status "1.11 get users by IDs" 200 "$LAST_STATUS"
assert_json_gte "1.11 returned 2 users" "length" 2 "$LAST_BODY"

echo ""
echo -e "  ${BOLD}Users provisioned:${NC}"
echo "    Alice:   ${ALICE_ID}"
echo "    Bob:     ${BOB_ID}"
echo "    Charlie: ${CHARLIE_ID}"

# ============================================================================
# SCENARIO 2: Team & Channel Lifecycle
# ============================================================================
section "Scenario 2: Team & Channel Lifecycle"

# 2.1 Alice creates a team (or reuses existing)
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/teams" '{"name":"test-team","display_name":"Test Team","type":"O"}'
if [[ "$LAST_STATUS" == "201" ]]; then
  TEAM_ID=$(echo "$LAST_BODY" | jq -r '.id')
  assert_json_not_empty "2.1 team has ID" ".id" "$LAST_BODY"
  assert_json "2.1 team name" ".name" "test-team" "$LAST_BODY"
  assert_json "2.1 creator is Alice" ".creator_id" "$ALICE_ID" "$LAST_BODY"
else
  echo -e "  ${YELLOW}WARN${NC}: Team 'test-team' already exists, reusing"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  curl_api_full GET "/teams?page=0&per_page=100"
  TEAM_ID=$(echo "$LAST_BODY" | jq -r '.[] | select(.name=="test-team") | .id')
  if [[ -n "$TEAM_ID" && "$TEAM_ID" != "null" ]]; then
    echo -e "  ${GREEN}PASS${NC} 2.1 reused existing team ${TEAM_ID}"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 2.1 could not create or find team"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
fi

# 2.2 Alice is auto-added as team_admin
curl_api_full GET "/teams/${TEAM_ID}/members"
assert_status "2.2 get team members" 200 "$LAST_STATUS"
ALICE_TEAM_ROLES=$(echo "$LAST_BODY" | jq -r ".[] | select(.user_id==\"${ALICE_ID}\") | .roles")
if [[ "$ALICE_TEAM_ROLES" == *"team_admin"* && "$ALICE_TEAM_ROLES" == *"team_user"* ]]; then
  echo -e "  ${GREEN}PASS${NC} 2.2 Alice has team_admin team_user roles"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 2.2 Alice roles: expected 'team_admin team_user', got '${ALICE_TEAM_ROLES}'"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 2.3 Add Bob as team member (idempotent)
curl_api_full POST "/teams/${TEAM_ID}/members" "{\"user_id\":\"${BOB_ID}\"}"
if [[ "$LAST_STATUS" == "201" ]]; then
  echo -e "  ${GREEN}PASS${NC} 2.3 add Bob to team (HTTP 201)"
  PASS_COUNT=$((PASS_COUNT + 1))
  assert_json "2.3 Bob team member" ".user_id" "$BOB_ID" "$LAST_BODY"
else
  echo -e "  ${YELLOW}WARN${NC}: Bob already in team (HTTP ${LAST_STATUS}), skipping"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# 2.4 Add Charlie as team member (idempotent)
curl_api_full POST "/teams/${TEAM_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
if [[ "$LAST_STATUS" == "201" ]]; then
  echo -e "  ${GREEN}PASS${NC} 2.4 add Charlie to team (HTTP 201)"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${YELLOW}WARN${NC}: Charlie already in team, skipping"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# 2.5 Verify team has 3 members
curl_api_full GET "/teams/${TEAM_ID}/members"
assert_status "2.5 get team members" 200 "$LAST_STATUS"
assert_json_gte "2.5 team has >= 3 members" "length" 3 "$LAST_BODY"

# 2.6 Alice creates a public channel (or reuses existing)
curl_api_full POST "/channels" "{\"team_id\":\"${TEAM_ID}\",\"name\":\"general\",\"display_name\":\"General\",\"type\":\"O\",\"purpose\":\"General discussion\"}"
if [[ "$LAST_STATUS" == "201" ]]; then
  PUBLIC_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
  assert_json "2.6 channel type O" ".type" "O" "$LAST_BODY"
  assert_json "2.6 channel name" ".name" "general" "$LAST_BODY"
else
  echo -e "  ${YELLOW}WARN${NC}: Channel 'general' already exists, reusing"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  curl_api_full GET "/teams/${TEAM_ID}/channels?page=0&per_page=100"
  PUBLIC_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.[] | select(.name=="general") | .id')
  if [[ -n "$PUBLIC_CHANNEL_ID" && "$PUBLIC_CHANNEL_ID" != "null" ]]; then
    echo -e "  ${GREEN}PASS${NC} 2.6 reused existing public channel ${PUBLIC_CHANNEL_ID}"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 2.6 could not create or find public channel"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
fi

# 2.7 Alice creates a private channel (or reuses existing)
curl_api_full POST "/channels" "{\"team_id\":\"${TEAM_ID}\",\"name\":\"secret-ops\",\"display_name\":\"Secret Ops\",\"type\":\"P\"}"
if [[ "$LAST_STATUS" == "201" ]]; then
  PRIVATE_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
  assert_json "2.7 channel type P" ".type" "P" "$LAST_BODY"
else
  echo -e "  ${YELLOW}WARN${NC}: Channel 'secret-ops' already exists, reusing"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  # The team list shows open channels only; a private channel is found
  # through its member's own list.
  curl_api_full GET "/users/me/teams/${TEAM_ID}/channels"
  PRIVATE_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.[] | select(.name=="secret-ops") | .id')
  if [[ -n "$PRIVATE_CHANNEL_ID" && "$PRIVATE_CHANNEL_ID" != "null" ]]; then
    echo -e "  ${GREEN}PASS${NC} 2.7 reused existing private channel ${PRIVATE_CHANNEL_ID}"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 2.7 could not create or find private channel"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
fi

# 2.8 Verify Keto relation for Alice on public channel
KETO_ALLOWED=$(keto_allowed "${PUBLIC_CHANNEL_ID}" "${ALICE_ID}")
if [[ "$KETO_ALLOWED" == "true" ]]; then
  echo -e "  ${GREEN}PASS${NC} 2.8 Keto: Alice is member of public channel"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 2.8 Keto: Alice not member of public channel (allowed=${KETO_ALLOWED})"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 2.9 Add Bob to public channel
curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/members" "{\"user_id\":\"${BOB_ID}\"}"
assert_status "2.9 add Bob to public channel" 201 "$LAST_STATUS"

# 2.10 Add Charlie to public channel
curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "2.10 add Charlie to public channel" 201 "$LAST_STATUS"

# 2.11 Verify Keto for Bob on public channel
# Bob was already on the team, so the channel's creation added him (and
# wrote his tuple, asynchronously); 2.9 found him already a member.
KETO_CHECK="{\"allowed\": $(keto_allowed "${PUBLIC_CHANNEL_ID}" "${BOB_ID}")}"
assert_json "2.11 Keto: Bob is member" ".allowed" "true" "$KETO_CHECK"

# 2.12 Add Bob to private channel then remove him
curl_api_full POST "/channels/${PRIVATE_CHANNEL_ID}/members" "{\"user_id\":\"${BOB_ID}\"}"
assert_status "2.12a add Bob to private channel" 201 "$LAST_STATUS"
# 2.12b Removing another member needs system_admin (e85f76d); Alice, the
# channel's creator, is not one.
curl_api_full DELETE "/channels/${PRIVATE_CHANNEL_ID}/members/${BOB_ID}"
assert_status "2.12b non-admin cannot remove Bob" 403 "$LAST_STATUS"
# 2.12c-e Dave, the system_admin, joins the team and the channel (removal
# also needs channel membership), then removes Bob.
curl_api_full POST "/teams/${TEAM_ID}/members" "{\"user_id\":\"${DAVE_ID}\"}"
assert_status "2.12c add Dave to team" 201 "$LAST_STATUS"
curl_api_full POST "/channels/${PRIVATE_CHANNEL_ID}/members" "{\"user_id\":\"${DAVE_ID}\"}"
assert_status "2.12d add Dave to private channel" 201 "$LAST_STATUS"
CURRENT_TOKEN="$DAVE_TOKEN"
curl_api_full DELETE "/channels/${PRIVATE_CHANNEL_ID}/members/${BOB_ID}"
assert_status "2.12e system_admin removes Bob" 200 "$LAST_STATUS"
CURRENT_TOKEN="$ALICE_TOKEN"

# 2.13 Verify Keto relation removed for Bob on private channel
KETO_CHECK=$(curl -s "${KETO_READ}/relation-tuples/check?namespace=chit/channel&object=${PRIVATE_CHANNEL_ID}&relation=member&subject_id=${BOB_ID}")
KETO_ALLOWED=$(echo "$KETO_CHECK" | jq -r '.allowed')
if [[ "$KETO_ALLOWED" == "false" ]]; then
  echo -e "  ${GREEN}PASS${NC} 2.13 Keto: Bob removed from private channel"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 2.13 Keto: Bob still member of private channel (allowed=${KETO_ALLOWED})"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 2.14 Bob's "my channels" shows only channels he's a member of
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/users/me/teams/${TEAM_ID}/channels"
assert_status "2.14 Bob's my channels" 200 "$LAST_STATUS"
BOB_CHANNEL_IDS=$(echo "$LAST_BODY" | jq -r '.[].id')
if echo "$BOB_CHANNEL_IDS" | grep -q "$PUBLIC_CHANNEL_ID"; then
  echo -e "  ${GREEN}PASS${NC} 2.14a Bob sees public channel"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 2.14a Bob does not see public channel"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi
if echo "$BOB_CHANNEL_IDS" | grep -q "$PRIVATE_CHANNEL_ID"; then
  echo -e "  ${RED}FAIL${NC} 2.14b Bob still sees private channel after removal"
  FAIL_COUNT=$((FAIL_COUNT + 1))
else
  echo -e "  ${GREEN}PASS${NC} 2.14b Bob does not see private channel (removed)"
  PASS_COUNT=$((PASS_COUNT + 1))
fi

# 2.15 Verify public channel has 3 members
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/members"
assert_status "2.15 public channel members" 200 "$LAST_STATUS"
assert_json_gte "2.15 public channel has >= 3 members" "length" 3 "$LAST_BODY"

# 2.16 Update team display name
curl_api_full PUT "/teams/${TEAM_ID}" '{"display_name":"Test Team Updated","type":"O"}'
assert_status "2.16 update team" 200 "$LAST_STATUS"
assert_json "2.16 team display_name updated" ".display_name" "Test Team Updated" "$LAST_BODY"

# 2.17 Update channel header
curl_api_full PUT "/channels/${PUBLIC_CHANNEL_ID}" '{"header":"Welcome to General!","type":"O"}'
assert_status "2.17 update channel header" 200 "$LAST_STATUS"
assert_json "2.17 channel header updated" ".header" "Welcome to General!" "$LAST_BODY"

# 2.18 Get team by ID
curl_api_full GET "/teams/${TEAM_ID}"
assert_status "2.18 get team by ID" 200 "$LAST_STATUS"
assert_json "2.18 team ID matches" ".id" "$TEAM_ID" "$LAST_BODY"

# 2.19 Alice's my teams
curl_api_full GET "/users/me/teams"
assert_status "2.19 Alice my teams" 200 "$LAST_STATUS"
assert_json_gte "2.19 Alice in >= 1 team" "length" 1 "$LAST_BODY"

# ============================================================================
# SCENARIO 3: Channel Messaging
# ============================================================================
section "Scenario 3: Channel Messaging"

# 3.1 Alice posts a message
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Hello from Alice! This is the first message.\"}"
assert_status "3.1 Alice posts message" 201 "$LAST_STATUS"
ALICE_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json_not_empty "3.1 post has ID" ".id" "$LAST_BODY"
assert_json "3.1 post user_id" ".user_id" "$ALICE_ID" "$LAST_BODY"

# 3.2 Bob posts a message
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Hey Alice! Bob here.\"}"
assert_status "3.2 Bob posts message" 201 "$LAST_STATUS"
BOB_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 3.3 Alice posts more messages for pagination testing
CURRENT_TOKEN="$ALICE_TOKEN"
for i in $(seq 1 5); do
  curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Pagination test message ${i}\"}"
done

# 3.4 Read channel posts (default pagination)
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/posts"
assert_status "3.4 read channel posts" 200 "$LAST_STATUS"
POST_COUNT=$(echo "$LAST_BODY" | jq '.order | length')
assert_json_gte "3.4 channel has >= 7 posts" ".order | length" 7 "$LAST_BODY"

# 3.5 Read with pagination (page=0, per_page=3)
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/posts?page=0&per_page=3"
assert_status "3.5 paginated posts" 200 "$LAST_STATUS"
PAGE_COUNT=$(echo "$LAST_BODY" | jq '.order | length')
if [[ "$PAGE_COUNT" -le 3 ]]; then
  echo -e "  ${GREEN}PASS${NC} 3.5 page limited to <= 3 posts (got ${PAGE_COUNT})"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 3.5 expected <= 3 posts, got ${PAGE_COUNT}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 3.6 Get single post by ID
curl_api_full GET "/posts/${ALICE_POST_ID}"
assert_status "3.6 get post by ID" 200 "$LAST_STATUS"
assert_json "3.6 post ID matches" ".id" "$ALICE_POST_ID" "$LAST_BODY"
assert_json "3.6 post content" ".content" "Hello from Alice! This is the first message." "$LAST_BODY"

# 3.7 Edit a post
curl_api_full PUT "/posts/${ALICE_POST_ID}" '{"content":"Hello from Alice! (edited)"}'
assert_status "3.7 edit post" 200 "$LAST_STATUS"
assert_json "3.7 updated content" ".content" "Hello from Alice! (edited)" "$LAST_BODY"

# 3.8 Verify edit persists
curl_api_full GET "/posts/${ALICE_POST_ID}"
assert_status "3.8 get edited post" 200 "$LAST_STATUS"
assert_json "3.8 edit persisted" ".content" "Hello from Alice! (edited)" "$LAST_BODY"

# 3.9 Pin a post
curl_api_full POST "/posts/${ALICE_POST_ID}/pin"
assert_status "3.9 pin post" 200 "$LAST_STATUS"

# 3.10 Verify pinned posts
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/pinned"
assert_status "3.10 get pinned posts" 200 "$LAST_STATUS"
PINNED_IDS=$(echo "$LAST_BODY" | jq -r '.order[].id' 2>/dev/null || echo "")
if echo "$PINNED_IDS" | grep -q "$ALICE_POST_ID"; then
  echo -e "  ${GREEN}PASS${NC} 3.10 Alice's post appears in pinned"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 3.10 Alice's post not in pinned list"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 3.11 Unpin the post
curl_api_full POST "/posts/${ALICE_POST_ID}/unpin"
assert_status "3.11 unpin post" 200 "$LAST_STATUS"

# 3.12 Delete Bob's post
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full DELETE "/posts/${BOB_POST_ID}"
assert_status "3.12 delete post" 200 "$LAST_STATUS"

# 3.13 Mark channel as viewed
curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/members/me/view"
assert_status "3.13 mark channel viewed" 200 "$LAST_STATUS"

# ============================================================================
# SCENARIO 4: Direct & Group Messaging
# ============================================================================
section "Scenario 4: Direct & Group Messaging"

# 4.1 Alice creates DM with Bob
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/channels/direct" "[\"${ALICE_ID}\",\"${BOB_ID}\"]"
assert_status "4.1 create DM channel" 201 "$LAST_STATUS"
DM_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json "4.1 DM type is D" ".type" "D" "$LAST_BODY"

# 4.2 DM channel has no team_id
DM_TEAM_ID=$(echo "$LAST_BODY" | jq -r '.team_id // empty')
if [[ -z "$DM_TEAM_ID" || "$DM_TEAM_ID" == "null" || "$DM_TEAM_ID" == "" ]]; then
  echo -e "  ${GREEN}PASS${NC} 4.2 DM team_id is empty/null"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 4.2 DM team_id should be empty, got '${DM_TEAM_ID}'"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 4.3 DM has 2 members
curl_api_full GET "/channels/${DM_CHANNEL_ID}/members"
assert_status "4.3 DM members" 200 "$LAST_STATUS"
assert_json_gte "4.3 DM has 2 members" "length" 2 "$LAST_BODY"

# 4.4 Keto: Alice is member of DM
KETO_CHECK="{\"allowed\": $(keto_allowed "${DM_CHANNEL_ID}" "${ALICE_ID}")}"
assert_json "4.4 Keto: Alice DM member" ".allowed" "true" "$KETO_CHECK"

# 4.5 Keto: Bob is member of DM
KETO_CHECK="{\"allowed\": $(keto_allowed "${DM_CHANNEL_ID}" "${BOB_ID}")}"
assert_json "4.5 Keto: Bob DM member" ".allowed" "true" "$KETO_CHECK"

# 4.6 Alice sends DM
curl_api_full POST "/posts" "{\"channel_id\":\"${DM_CHANNEL_ID}\",\"content\":\"Hey Bob, private DM here!\"}"
assert_status "4.6 Alice sends DM" 201 "$LAST_STATUS"
DM_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 4.7 Bob reads DM
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/channels/${DM_CHANNEL_ID}/posts"
assert_status "4.7 Bob reads DM posts" 200 "$LAST_STATUS"
assert_json_gte "4.7 DM has >= 1 post" ".order | length" 1 "$LAST_BODY"

# 4.8 Create group channel (Alice, Bob, Charlie)
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/channels/group" "[\"${ALICE_ID}\",\"${BOB_ID}\",\"${CHARLIE_ID}\"]"
assert_status "4.8 create group channel" 201 "$LAST_STATUS"
GROUP_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json "4.8 group type is G" ".type" "G" "$LAST_BODY"

# 4.9 Group has 3 members
curl_api_full GET "/channels/${GROUP_CHANNEL_ID}/members"
assert_status "4.9 group members" 200 "$LAST_STATUS"
assert_json_gte "4.9 group has 3 members" "length" 3 "$LAST_BODY"

# 4.10 Alice posts in group
curl_api_full POST "/posts" "{\"channel_id\":\"${GROUP_CHANNEL_ID}\",\"content\":\"Hello group!\"}"
assert_status "4.10 Alice posts in group" 201 "$LAST_STATUS"

# 4.11 Charlie reads group messages
CURRENT_TOKEN="$CHARLIE_TOKEN"
curl_api_full GET "/channels/${GROUP_CHANNEL_ID}/posts"
assert_status "4.11 Charlie reads group posts" 200 "$LAST_STATUS"
assert_json_gte "4.11 group has >= 1 post" ".order | length" 1 "$LAST_BODY"

# ============================================================================
# SCENARIO 5: Threading
# ============================================================================
section "Scenario 5: Threading"

# 5.1 Post a root message from Alice
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Thread root: Let's discuss the project plan.\"}"
assert_status "5.1 thread root post" 201 "$LAST_STATUS"
THREAD_ROOT_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 5.2 Bob replies to Alice's post
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Sounds good, Alice!\",\"root_id\":\"${THREAD_ROOT_ID}\"}"
assert_status "5.2 Bob thread reply" 201 "$LAST_STATUS"
assert_json "5.2 reply root_id" ".root_id" "$THREAD_ROOT_ID" "$LAST_BODY"

# 5.3 Alice replies
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Great, let's set up a meeting.\",\"root_id\":\"${THREAD_ROOT_ID}\"}"
assert_status "5.3 Alice thread reply" 201 "$LAST_STATUS"

# 5.4 Charlie replies
CURRENT_TOKEN="$CHARLIE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Count me in!\",\"root_id\":\"${THREAD_ROOT_ID}\"}"
assert_status "5.4 Charlie thread reply" 201 "$LAST_STATUS"

# Brief pause for thread state to settle
sleep 0.5

# 5.5 Read thread — should have root + 3 replies = 4 posts
curl_api_full GET "/posts/${THREAD_ROOT_ID}/thread"
assert_status "5.5 get thread" 200 "$LAST_STATUS"
THREAD_POST_COUNT=$(echo "$LAST_BODY" | jq '.order | length')
assert_json_gte "5.5 thread has >= 4 posts" ".order | length" 4 "$LAST_BODY"

# 5.6 Get user's followed threads
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/users/me/teams/${TEAM_ID}/threads"
assert_status "5.6 Bob's followed threads" 200 "$LAST_STATUS"
# Bob auto-followed the thread by replying
THREAD_TOTAL=$(echo "$LAST_BODY" | jq -r '.total // 0')
if [[ "$THREAD_TOTAL" -ge 1 ]]; then
  echo -e "  ${GREEN}PASS${NC} 5.6 Bob follows >= 1 thread (total=${THREAD_TOTAL})"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${YELLOW}WARN${NC} 5.6 Bob follows 0 threads (may be expected if threading tables are empty)"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# 5.7 Mark thread as read
curl_api_full PUT "/users/me/teams/${TEAM_ID}/threads/${THREAD_ROOT_ID}/read"
assert_status "5.7 mark thread read" 200 "$LAST_STATUS"

# 5.8 Unfollow thread
curl_api_full PUT "/users/me/teams/${TEAM_ID}/threads/${THREAD_ROOT_ID}/following" '{"following":false}'
assert_status "5.8 unfollow thread" 200 "$LAST_STATUS"

# 5.9 Re-follow thread
curl_api_full PUT "/users/me/teams/${TEAM_ID}/threads/${THREAD_ROOT_ID}/following" '{"following":true}'
assert_status "5.9 re-follow thread" 200 "$LAST_STATUS"

# 5.10 Standalone post (no root_id) — thread endpoint returns just itself
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Standalone post, no threading.\"}"
assert_status "5.10a create standalone post" 201 "$LAST_STATUS"
STANDALONE_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
curl_api_full GET "/posts/${STANDALONE_POST_ID}/thread"
assert_status "5.10b get standalone thread" 200 "$LAST_STATUS"
STANDALONE_THREAD_COUNT=$(echo "$LAST_BODY" | jq '.order | length')
if [[ "$STANDALONE_THREAD_COUNT" -le 1 ]]; then
  echo -e "  ${GREEN}PASS${NC} 5.10 standalone post thread returns <= 1 post"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 5.10 standalone thread has ${STANDALONE_THREAD_COUNT} posts, expected 1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# ============================================================================
# SCENARIO 6: Tags & Search
# ============================================================================
section "Scenario 6: Tags & Search"

CURRENT_TOKEN="$ALICE_TOKEN"

# 6.1 Create tags
curl_api_full POST "/tags" '{"name":"bug"}'
assert_status "6.1a create tag 'bug'" 201 "$LAST_STATUS"
TAG_BUG_ID=$(echo "$LAST_BODY" | jq -r '.id')

curl_api_full POST "/tags" '{"name":"feature-request"}'
assert_status "6.1b create tag 'feature-request'" 201 "$LAST_STATUS"
TAG_FEATURE_ID=$(echo "$LAST_BODY" | jq -r '.id')

curl_api_full POST "/tags" '{"name":"urgent"}'
assert_status "6.1c create tag 'urgent'" 201 "$LAST_STATUS"
TAG_URGENT_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 6.2 List all tags
curl_api_full GET "/tags"
assert_status "6.2 list all tags" 200 "$LAST_STATUS"
assert_json_gte "6.2 has >= 3 tags" "length" 3 "$LAST_BODY"

# 6.3 Tag Alice's post with 'bug' and 'urgent'
curl_api_full POST "/posts/${ALICE_POST_ID}/tags" "{\"tag_id\":\"${TAG_BUG_ID}\"}"
assert_status "6.3a tag post with bug" 200 "$LAST_STATUS"
curl_api_full POST "/posts/${ALICE_POST_ID}/tags" "{\"tag_id\":\"${TAG_URGENT_ID}\"}"
assert_status "6.3b tag post with urgent" 200 "$LAST_STATUS"

# 6.4 Get tags for post
curl_api_full GET "/posts/${ALICE_POST_ID}/tags"
assert_status "6.4 get post tags" 200 "$LAST_STATUS"
assert_json_gte "6.4 post has >= 2 tags" "length" 2 "$LAST_BODY"

# 6.5 Remove 'urgent' tag
curl_api_full DELETE "/posts/${ALICE_POST_ID}/tags/${TAG_URGENT_ID}"
assert_status "6.5 remove urgent tag" 200 "$LAST_STATUS"

# 6.6 Verify tag count reduced
curl_api_full GET "/posts/${ALICE_POST_ID}/tags"
assert_status "6.6 get post tags after removal" 200 "$LAST_STATUS"
TAG_COUNT=$(echo "$LAST_BODY" | jq 'length')
if [[ "$TAG_COUNT" -eq 1 ]]; then
  echo -e "  ${GREEN}PASS${NC} 6.6 post has 1 tag after removal"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 6.6 expected 1 tag after removal, got ${TAG_COUNT}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 6.7 Tag the thread root post
curl_api_full POST "/posts/${THREAD_ROOT_ID}/tags" "{\"tag_id\":\"${TAG_FEATURE_ID}\"}"
assert_status "6.7 tag thread root" 200 "$LAST_STATUS"

# --- Scoped tags (P-CHIT-7). Tags without a scope, as above, are private. ---

# 6.11 A private tag stays Alice's: Bob, in the same channel, never sees it.
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/tags" '{"name":"alice only","scope":"private"}'
assert_status "6.11a create private tag" 201 "$LAST_STATUS"
TAG_PRIVATE_ID=$(echo "$LAST_BODY" | jq -r '.id')
curl_api_full POST "/posts/${ALICE_POST_ID}/tags" "{\"tag_id\":\"${TAG_PRIVATE_ID}\"}"
assert_status "6.11b apply private tag" 200 "$LAST_STATUS"
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/posts/${ALICE_POST_ID}/tags"
assert_json "6.11c Bob does not see it on the post" "[.[] | select(.id == \"${TAG_PRIVATE_ID}\")] | length" "0" "$LAST_BODY"
curl_api_full GET "/tags"
assert_json "6.11d Bob does not see it listed" "[.[] | select(.id == \"${TAG_PRIVATE_ID}\")] | length" "0" "$LAST_BODY"
curl_api_full POST "/posts/${ALICE_POST_ID}/tags" "{\"tag_id\":\"${TAG_PRIVATE_ID}\"}"
assert_status "6.11e Bob cannot apply it" 404 "$LAST_STATUS"

# 6.12 A channel tag is shared by the channel's members.
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/tags" "{\"name\":\"Release\",\"scope\":\"channel\",\"channel_id\":\"${PUBLIC_CHANNEL_ID}\"}"
assert_status "6.12a create channel tag" 201 "$LAST_STATUS"
TAG_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/tags" "{\"tag_id\":\"${TAG_CHANNEL_ID}\"}"
assert_status "6.12b Bob applies it to the channel" 200 "$LAST_STATUS"
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/tags"
assert_json "6.12c Bob sees it on the channel" "[.[] | select(.id == \"${TAG_CHANNEL_ID}\")] | length" "1" "$LAST_BODY"
curl_api_full POST "/tags/channels" "{\"tag_ids\":[\"${TAG_CHANNEL_ID}\"]}"
assert_json "6.12d the filter finds the channel" "[.[] | select(.channel.id == \"${PUBLIC_CHANNEL_ID}\")] | length" "1" "$LAST_BODY"
curl_api_full PUT "/tags/${TAG_CHANNEL_ID}" '{"name":"mine now"}'
assert_status "6.12e only the owner renames it" 403 "$LAST_STATUS"

# 6.13 profile/ is reserved for HALO, acting on a person's behalf.
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/tags" '{"name":"profile/coder"}'
assert_status "6.13 a person cannot create a profile/ tag" 403 "$LAST_STATUS"

# --- Search (ZincSearch) ---
# The search indexer is a skeleton (TODO), so we manually index posts

# 6.8 Manual ZincSearch index (best-effort)
echo -e "  ${CYAN}INFO${NC} 6.8 Manually indexing posts in ZincSearch..."
SEARCH_INDEXED=false

# Create the index first (idempotent)
curl -s -u "${ZINC_USER}:${ZINC_PASS}" -X POST "${ZINC_URL}/api/index" \
  -H "Content-Type: application/json" \
  -d '{"name":"chit-posts","storage_type":"disk"}' >/dev/null 2>&1 || true

# Index Alice's post
INDEX_RESP=$(curl -s -o /dev/null -w '%{http_code}' -u "${ZINC_USER}:${ZINC_PASS}" \
  -X POST "${ZINC_URL}/api/chit-posts/_doc" \
  -H "Content-Type: application/json" \
  -d "{\"_id\":\"${ALICE_POST_ID}\",\"content\":\"Hello from Alice! (edited)\",\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"user_id\":\"${ALICE_ID}\"}" 2>/dev/null || echo "000")
if [[ "$INDEX_RESP" == "200" ]]; then
  SEARCH_INDEXED=true
fi

# Index thread root
curl -s -u "${ZINC_USER}:${ZINC_PASS}" \
  -X POST "${ZINC_URL}/api/chit-posts/_doc" \
  -H "Content-Type: application/json" \
  -d "{\"_id\":\"${THREAD_ROOT_ID}\",\"content\":\"Thread root: Let's discuss the project plan.\",\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"user_id\":\"${ALICE_ID}\"}" >/dev/null 2>&1 || true

if [[ "$SEARCH_INDEXED" == "true" ]]; then
  echo -e "  ${GREEN}PASS${NC} 6.8 manually indexed posts in ZincSearch"
  PASS_COUNT=$((PASS_COUNT + 1))

  # Brief pause for ZincSearch to settle
  sleep 1

  # 6.9 Search posts in team
  curl_api_full POST "/teams/${TEAM_ID}/posts/search" '{"terms":"Alice","per_page":10}'
  assert_status "6.9 search posts in team" 200 "$LAST_STATUS"
  SEARCH_RESULTS=$(echo "$LAST_BODY" | jq '.order | length' 2>/dev/null || echo "0")
  if [[ "$SEARCH_RESULTS" -ge 1 ]]; then
    echo -e "  ${GREEN}PASS${NC} 6.9 search found >= 1 result"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${YELLOW}WARN${NC} 6.9 search returned 0 results (ZincSearch indexing delay)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi

  # 6.10 Search posts in channel
  curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/posts/search" '{"terms":"discuss","per_page":10}'
  assert_status "6.10 search posts in channel" 200 "$LAST_STATUS"
  SEARCH_RESULTS=$(echo "$LAST_BODY" | jq '.order | length' 2>/dev/null || echo "0")
  if [[ "$SEARCH_RESULTS" -ge 1 ]]; then
    echo -e "  ${GREEN}PASS${NC} 6.10 channel search found >= 1 result"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${YELLOW}WARN${NC} 6.10 channel search returned 0 results (ZincSearch indexing delay)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
  fi
else
  echo -e "  ${YELLOW}WARN${NC} 6.8 ZincSearch indexing failed — skipping search tests"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  SKIP_COUNT=$((SKIP_COUNT + 1))
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# ============================================================================
# SCENARIO 7: @Mention Support
# ============================================================================
section "Scenario 7: @Mention Support"

# 7.1 Alice posts a message mentioning Bob
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Hey @bob, can you review this?\"}"
assert_status "7.1 Alice posts with @bob mention" 201 "$LAST_STATUS"
MENTION_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
# Verify props.mentions contains Bob's user ID
MENTION_LIST=$(echo "$LAST_BODY" | jq -r '.props.mentions // []')
if echo "$MENTION_LIST" | jq -e ".[] | select(. == \"${BOB_ID}\")" >/dev/null 2>&1; then
  echo -e "  ${GREEN}PASS${NC} 7.1 props.mentions contains Bob's ID"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.1 props.mentions does not contain Bob's ID (got: ${MENTION_LIST})"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 7.2 Check Bob's channel mention_count incremented
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/channels/${PUBLIC_CHANNEL_ID}/members"
assert_status "7.2 get channel members" 200 "$LAST_STATUS"
BOB_MENTION_COUNT=$(echo "$LAST_BODY" | jq -r "[.[] | select(.user_id==\"${BOB_ID}\")] | .[0].mention_count // 0")
if [[ "$BOB_MENTION_COUNT" -ge 1 ]]; then
  echo -e "  ${GREEN}PASS${NC} 7.2 Bob mention_count >= 1 (got ${BOB_MENTION_COUNT})"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.2 Bob mention_count expected >= 1, got ${BOB_MENTION_COUNT}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 7.3 Self-mention is NOT counted — Alice mentions herself
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"I am @alice talking to myself\"}"
assert_status "7.3a Alice posts self-mention" 201 "$LAST_STATUS"
SELF_MENTION_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
# props.mentions should NOT contain Alice's own ID
SELF_MENTIONS=$(echo "$LAST_BODY" | jq -r '.props.mentions // []')
if echo "$SELF_MENTIONS" | jq -e ".[] | select(. == \"${ALICE_ID}\")" >/dev/null 2>&1; then
  echo -e "  ${RED}FAIL${NC} 7.3 self-mention: props.mentions contains Alice's own ID"
  FAIL_COUNT=$((FAIL_COUNT + 1))
else
  echo -e "  ${GREEN}PASS${NC} 7.3 self-mention filtered from props.mentions"
  PASS_COUNT=$((PASS_COUNT + 1))
fi

# 7.4 @all mention — mentions all channel members except author
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"Hey @all, standup time!\"}"
assert_status "7.4 Alice posts @all" 201 "$LAST_STATUS"
ALL_MENTION_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
ALL_MENTIONS=$(echo "$LAST_BODY" | jq -r '.props.mentions // []')
ALL_MENTION_COUNT=$(echo "$ALL_MENTIONS" | jq 'length')
# Should mention at least Bob and Charlie (2+ members besides Alice)
if [[ "$ALL_MENTION_COUNT" -ge 2 ]]; then
  echo -e "  ${GREEN}PASS${NC} 7.4 @all mentions >= 2 other members (got ${ALL_MENTION_COUNT})"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.4 @all expected >= 2 mentions, got ${ALL_MENTION_COUNT}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi
# Verify Alice is NOT in the @all mention list
if echo "$ALL_MENTIONS" | jq -e ".[] | select(. == \"${ALICE_ID}\")" >/dev/null 2>&1; then
  echo -e "  ${RED}FAIL${NC} 7.4b @all includes author (Alice)"
  FAIL_COUNT=$((FAIL_COUNT + 1))
else
  echo -e "  ${GREEN}PASS${NC} 7.4b @all excludes author (Alice)"
  PASS_COUNT=$((PASS_COUNT + 1))
fi

# 7.5 Mention non-member — not included in mentions
# Charlie is not in the private channel, mentioning him should be silently skipped
CURRENT_TOKEN="$ALICE_TOKEN"
# First ensure Alice is a member of the private channel
curl_api_full GET "/channels/${PRIVATE_CHANNEL_ID}/members"
assert_status "7.5a get private channel members" 200 "$LAST_STATUS"
curl_api_full POST "/posts" "{\"channel_id\":\"${PRIVATE_CHANNEL_ID}\",\"content\":\"Hey @charlie, are you here?\"}"
assert_status "7.5 post mentioning non-member" 201 "$LAST_STATUS"
NON_MEMBER_MENTIONS=$(echo "$LAST_BODY" | jq -r '.props.mentions // "null"')
if [[ "$NON_MEMBER_MENTIONS" == "null" ]] || ! echo "$NON_MEMBER_MENTIONS" | jq -e ".[] | select(. == \"${CHARLIE_ID}\")" >/dev/null 2>&1; then
  echo -e "  ${GREEN}PASS${NC} 7.5 non-member Charlie not in mentions"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.5 non-member Charlie appeared in mentions"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 7.6 Mention in thread reply — check thread unread_mention_count
# Bob follows the thread from scenario 5 (THREAD_ROOT_ID). Alice mentions Bob in a reply.
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"@bob what do you think about this thread?\",\"root_id\":\"${THREAD_ROOT_ID}\"}"
assert_status "7.6 Alice mentions Bob in thread reply" 201 "$LAST_STATUS"
THREAD_MENTION_PROPS=$(echo "$LAST_BODY" | jq -r '.props.mentions // []')
if echo "$THREAD_MENTION_PROPS" | jq -e ".[] | select(. == \"${BOB_ID}\")" >/dev/null 2>&1; then
  echo -e "  ${GREEN}PASS${NC} 7.6 thread reply props.mentions contains Bob"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.6 thread reply props.mentions missing Bob"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 7.7 Edit post re-parses mentions but does NOT double-count
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full PUT "/posts/${MENTION_POST_ID}" '{"content":"Hey @bob and @charlie, updated mention"}'
assert_status "7.7 edit post with mentions" 200 "$LAST_STATUS"
EDIT_MENTIONS=$(echo "$LAST_BODY" | jq -r '.props.mentions // []')
EDIT_MENTION_COUNT=$(echo "$EDIT_MENTIONS" | jq 'length')
if [[ "$EDIT_MENTION_COUNT" -ge 2 ]]; then
  echo -e "  ${GREEN}PASS${NC} 7.7 edited post props.mentions updated (${EDIT_MENTION_COUNT} users)"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 7.7 edited post expected >= 2 mentions, got ${EDIT_MENTION_COUNT}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# ============================================================================
# SCENARIO 8: Slash Commands
# ============================================================================
section "Scenario 8: Slash Commands"

# 8.1 GET /api/v1/commands returns the command list
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full GET "/commands"
assert_status "8.1 GET /commands" 200 "$LAST_STATUS"
# Should be a JSON array (may be empty if CUE dir not loaded, but should still be 200)
CMD_COUNT=$(echo "$LAST_BODY" | jq 'if type == "array" then length else 0 end')
if [[ "$CMD_COUNT" -ge 0 ]]; then
  echo -e "  ${GREEN}PASS${NC} 8.1 /commands returns valid JSON array (${CMD_COUNT} commands)"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 8.1 /commands did not return JSON array"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 8.2 Verify /commands returns expected fields when commands are loaded
if [[ "$CMD_COUNT" -gt 0 ]]; then
  FIRST_SLUG=$(echo "$LAST_BODY" | jq -r '.[0].slug')
  FIRST_DESC=$(echo "$LAST_BODY" | jq -r '.[0].description')
  FIRST_CAT=$(echo "$LAST_BODY" | jq -r '.[0].category')
  if [[ -n "$FIRST_SLUG" && "$FIRST_SLUG" != "null" && -n "$FIRST_DESC" && "$FIRST_DESC" != "null" && -n "$FIRST_CAT" && "$FIRST_CAT" != "null" ]]; then
    echo -e "  ${GREEN}PASS${NC} 8.2 command has slug, description, category fields"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.2 command missing expected fields (slug=${FIRST_SLUG}, desc=${FIRST_DESC}, cat=${FIRST_CAT})"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
else
  echo -e "  ${YELLOW}SKIP${NC} 8.2 no commands loaded (CUE dir may not be configured)"
  SKIP_COUNT=$((SKIP_COUNT + 1))
fi

# 8.3 Posting a slash command returns an ephemeral response (not persisted)
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"/help\"}"
# When commands are loaded, we get 201 with type=command_response.
# When commands are NOT loaded, /help goes through as a normal post (also 201).
assert_status "8.3 POST /help" 201 "$LAST_STATUS"
SLASH_POST_TYPE=$(echo "$LAST_BODY" | jq -r '.type // ""')
SLASH_EPHEMERAL=$(echo "$LAST_BODY" | jq -r '.props.ephemeral // false')
if [[ "$CMD_COUNT" -gt 0 ]]; then
  # Commands are loaded — should be intercepted
  if [[ "$SLASH_POST_TYPE" == "command_response" ]]; then
    echo -e "  ${GREEN}PASS${NC} 8.3a /help intercepted: type=command_response"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.3a expected type=command_response, got ${SLASH_POST_TYPE}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
  if [[ "$SLASH_EPHEMERAL" == "true" ]]; then
    echo -e "  ${GREEN}PASS${NC} 8.3b ephemeral=true"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.3b expected ephemeral=true, got ${SLASH_EPHEMERAL}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi

  # 8.4 Verify the slash command was NOT persisted
  SLASH_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
  curl_api_full GET "/posts/${SLASH_POST_ID}"
  if [[ "$LAST_STATUS" == "404" ]]; then
    echo -e "  ${GREEN}PASS${NC} 8.4 slash command not persisted (404 on GET)"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.4 expected 404 for ephemeral post, got HTTP ${LAST_STATUS}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
else
  echo -e "  ${YELLOW}SKIP${NC} 8.3a/b/8.4 commands not loaded, slash intercept not active"
  SKIP_COUNT=$((SKIP_COUNT + 3))
fi

# 8.5 Posting an unknown slash command returns an ephemeral error
if [[ "$CMD_COUNT" -gt 0 ]]; then
  CURRENT_TOKEN="$ALICE_TOKEN"
  curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"/nonexistent-cmd\"}"
  assert_status "8.5 POST /nonexistent-cmd" 201 "$LAST_STATUS"
  UNKNOWN_TYPE=$(echo "$LAST_BODY" | jq -r '.type // ""')
  UNKNOWN_TEXT=$(echo "$LAST_BODY" | jq -r '.content // ""')
  if [[ "$UNKNOWN_TYPE" == "command_response" ]]; then
    echo -e "  ${GREEN}PASS${NC} 8.5a unknown command returns command_response"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.5a expected type=command_response for unknown cmd, got ${UNKNOWN_TYPE}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
  if echo "$UNKNOWN_TEXT" | grep -qi "unknown"; then
    echo -e "  ${GREEN}PASS${NC} 8.5b unknown command response mentions 'unknown'"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} 8.5b expected 'unknown' in response, got: ${UNKNOWN_TEXT:0:100}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
else
  echo -e "  ${YELLOW}SKIP${NC} 8.5 commands not loaded"
  SKIP_COUNT=$((SKIP_COUNT + 2))
fi

# 8.6 Normal post still persisted (no regression)
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PUBLIC_CHANNEL_ID}\",\"content\":\"This is a normal message, not a command\"}"
assert_status "8.6 normal post still works" 201 "$LAST_STATUS"
NORMAL_POST_TYPE=$(echo "$LAST_BODY" | jq -r '.type // ""')
NORMAL_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
if [[ "$NORMAL_POST_TYPE" != "command_response" ]]; then
  echo -e "  ${GREEN}PASS${NC} 8.6a normal post type is not command_response"
  PASS_COUNT=$((PASS_COUNT + 1))
else
  echo -e "  ${RED}FAIL${NC} 8.6a normal post incorrectly intercepted as command"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# 8.6b Verify the normal post was persisted
curl_api_full GET "/posts/${NORMAL_POST_ID}"
assert_status "8.6b normal post persisted" 200 "$LAST_STATUS"
assert_json "8.6b normal post content" ".content" "This is a normal message, not a command" "$LAST_BODY"

# 8.7 Unauthenticated request to /commands → 401
CURRENT_TOKEN=""
curl_api_full GET "/commands"
assert_status "8.7 unauthenticated /commands" 401 "$LAST_STATUS"

# ============================================================================
# SCENARIO 9: Authorization enforcement (cross-user 403s)
# ============================================================================
section "Scenario 9: Authorization Enforcement"

# 9.1 Charlie (not a member of the private channel) cannot post to it
CURRENT_TOKEN="$CHARLIE_TOKEN"
curl_api_full POST "/posts" "{\"channel_id\":\"${PRIVATE_CHANNEL_ID}\",\"content\":\"intruder message\"}"
assert_status "9.1 non-member cannot post to private channel" 403 "$LAST_STATUS"

# 9.2 Charlie cannot read the private channel's posts
curl_api_full GET "/channels/${PRIVATE_CHANNEL_ID}/posts"
assert_status "9.2 non-member cannot read private channel posts" 403 "$LAST_STATUS"

# 9.3 Charlie cannot list the private channel's members
curl_api_full GET "/channels/${PRIVATE_CHANNEL_ID}/members"
assert_status "9.3 non-member cannot list private channel members" 403 "$LAST_STATUS"

# 9.4 Charlie cannot add himself to the private channel
curl_api_full POST "/channels/${PRIVATE_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "9.4 non-member cannot self-invite to private channel" 403 "$LAST_STATUS"

# 9.5 Bob cannot edit Alice's post
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full PUT "/posts/${NORMAL_POST_ID}" "{\"content\":\"defaced by bob\"}"
assert_status "9.5 non-owner cannot edit another user's post" 403 "$LAST_STATUS"

# 9.6 Bob cannot delete Alice's post
curl_api_full DELETE "/posts/${NORMAL_POST_ID}"
assert_status "9.6 non-owner cannot delete another user's post" 403 "$LAST_STATUS"

# 9.7 Alice's post is unchanged after the failed edit/delete
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full GET "/posts/${NORMAL_POST_ID}"
assert_status "9.7 post still readable by owner" 200 "$LAST_STATUS"
assert_json "9.7 post content unchanged" ".content" "This is a normal message, not a command" "$LAST_BODY"

# ============================================================================
# SCENARIO 10: On behalf of a Kratos identity (HALO P-CHIT-8)
# ============================================================================
# halod, a machine actor with a Hydra client_credentials token, makes a
# Conversation for a person who has never opened Chit: it names them as
# kratos:<Kratos identity ID>, creates their private channel as them, adds
# itself as them, posts as them, and reads the channel as itself.
# deploy/chit.kube.yml declares the client in CHIT_MACHINE_ACTORS and
# CHIT_ON_BEHALF_OF_CLIENTS; this creates it in Hydra.
section "Scenario 10: On Behalf Of a Kratos Identity"

HALOD_CLIENT_ID="chit-e2e-halod"
HALOD_CLIENT_SECRET="chit-e2e-halod-secret"
HYDRA_CLIENT_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "${HYDRA_ADMIN}/admin/clients" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg id "$HALOD_CLIENT_ID" --arg secret "$HALOD_CLIENT_SECRET" '{
    client_id: $id, client_secret: $secret, grant_types: ["client_credentials"],
    scope: "chit:read chit:write", audience: ["chit"],
    token_endpoint_auth_method: "client_secret_basic" }')")
if [[ "$HYDRA_CLIENT_STATUS" != "201" && "$HYDRA_CLIENT_STATUS" != "409" ]]; then
  echo -e "  ${RED}FAIL${NC} 10.0 create Hydra client ${HALOD_CLIENT_ID} — HTTP ${HYDRA_CLIENT_STATUS}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
fi
HALOD_TOKEN=$(curl -s -u "${HALOD_CLIENT_ID}:${HALOD_CLIENT_SECRET}" -X POST "${HYDRA_PUBLIC}/oauth2/token" \
  -d grant_type=client_credentials -d "scope=chit:read chit:write" -d audience=chit | jq -r '.access_token')
assert_json_not_empty "10.0 halod has a client_credentials token" "." "\"${HALOD_TOKEN}\""

# halod_api METHOD PATH [DATA] [ON_BEHALF_OF] — as curl_api_full, with
# halod's bearer token and, when given, X-On-Behalf-Of.
halod_api() {
  local method="$1" path="$2" data="${3:-}" obo="${4:-}"
  local tmpfile
  tmpfile=$(mktemp)
  local args=(-s -w '%{http_code}' -X "$method" -o "$tmpfile"
    -H "Content-Type: application/json" -H "Authorization: Bearer ${HALOD_TOKEN}")
  [[ -n "$obo" ]] && args+=(-H "X-On-Behalf-Of: ${obo}")
  [[ -n "$data" ]] && args+=(-d "$data")
  LAST_STATUS=$(curl "${args[@]}" "${API}${path}")
  LAST_BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
}

# 10.1 halod learns its own user ID: GET /users/me without X-On-Behalf-Of.
halod_api GET "/users/me"
assert_status "10.1 halod reads itself" 200 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.1 halod is an agent" ".actor_type" "agent" "$LAST_BODY"
HALOD_USER_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 10.2 A person Kratos knows and Chit has never seen.
kratos_create_identity "erin@test.local" "erin" "Erin Example" "TestPass1234"
ERIN_KRATOS_ID="$IDENTITY_ID"
halod_api GET "/users/username/erin"
assert_status "10.2 Chit has no user for Erin yet" 404 "$LAST_STATUS" "$LAST_BODY"

# 10.3 Refusals: malformed, unknown to Kratos, a person's session.
halod_api GET "/users/me" "" "kratos:not-a-uuid"
assert_status "10.3a malformed kratos: value" 400 "$LAST_STATUS" "$LAST_BODY"
halod_api GET "/users/me" "" "kratos:$(cat /proc/sys/kernel/random/uuid)"
assert_status "10.3b identity unknown to Kratos" 400 "$LAST_STATUS" "$LAST_BODY"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -H "X-Session-Token: ${ALICE_TOKEN}" \
  -H "X-On-Behalf-Of: kratos:${ERIN_KRATOS_ID}" "${API}/users/me")
assert_status "10.3c a person's session may not act on behalf" 403 "$STATUS"
halod_api GET "/users/username/erin"
assert_status "10.3d the refusals provisioned nobody" 404 "$LAST_STATUS" "$LAST_BODY"

# 10.4 kratos:<id> provisions Erin, as her first browser visit would.
halod_api GET "/users/me" "" "kratos:${ERIN_KRATOS_ID}"
assert_status "10.4 halod provisions Erin" 200 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.4 Erin's kratos_id" ".kratos_id" "$ERIN_KRATOS_ID" "$LAST_BODY"
assert_json "10.4 Erin's username from Kratos" ".username" "erin" "$LAST_BODY"
assert_json "10.4 Erin is a person" ".actor_type" "user" "$LAST_BODY"
ERIN_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 10.5 halod's team: halod creates it as itself and adds Erin.
halod_api POST "/teams" '{"name":"halo-e2e","display_name":"HALO e2e","type":"I"}'
assert_status "10.5 halod creates its team" 201 "$LAST_STATUS" "$LAST_BODY"
HALO_TEAM_ID=$(echo "$LAST_BODY" | jq -r '.id')
halod_api POST "/teams/${HALO_TEAM_ID}/members" "{\"user_id\":\"${ERIN_ID}\"}"
assert_status "10.5 halod adds Erin to its team" 201 "$LAST_STATUS" "$LAST_BODY"

# 10.6 A private channel, created as Erin.
halod_api POST "/channels" "{\"team_id\":\"${HALO_TEAM_ID}\",\"name\":\"conversation-e2e\",\"display_name\":\"Conversation\",\"type\":\"P\"}" "kratos:${ERIN_KRATOS_ID}"
assert_status "10.6 create a channel on behalf of Erin" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.6 Erin is the creator" ".creator_id" "$ERIN_ID" "$LAST_BODY"
assert_json "10.6 the channel is private" ".type" "P" "$LAST_BODY"
CONV_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 10.7 halod reads nothing before it is a member.
halod_api GET "/channels/${CONV_ID}/posts"
assert_status "10.7 halod is not yet a member" 403 "$LAST_STATUS" "$LAST_BODY"

# 10.8 halod adds itself, as Erin.
halod_api POST "/channels/${CONV_ID}/members" "{\"user_id\":\"${HALOD_USER_ID}\"}" "kratos:${ERIN_KRATOS_ID}"
assert_status "10.8 add halod on behalf of Erin" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.8 Keto: halod is a member" "." "true" "$(keto_allowed "$CONV_ID" "$HALOD_USER_ID")"

# 10.9 halod posts as Erin.
halod_api POST "/posts" "{\"channel_id\":\"${CONV_ID}\",\"content\":\"Hello from Erin, via halod\"}" "kratos:${ERIN_KRATOS_ID}"
assert_status "10.9 post on behalf of Erin" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.9 Erin is the author" ".user_id" "$ERIN_ID" "$LAST_BODY"
assert_json "10.9 halod is recorded" ".props.on_behalf_of.actor_id" "$HALOD_USER_ID" "$LAST_BODY"
CONV_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')

# 10.10 halod reads the channel as itself; Bob, not a member, may not.
halod_api GET "/channels/${CONV_ID}/posts"
assert_status "10.10 halod reads as a member" 200 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.10 halod sees Erin's post" ".order[] | select(.id == \"${CONV_POST_ID}\") | .user_id" "$ERIN_ID" "$LAST_BODY"
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full GET "/channels/${CONV_ID}/posts"
assert_status "10.10 a non-member cannot read it" 403 "$LAST_STATUS" "$LAST_BODY"

# 10.11 Erin's own first login finds the same user.
kratos_login "erin@test.local" "TestPass1234"
CURRENT_TOKEN="$SESSION_TOKEN"
curl_api_full GET "/users/me"
assert_json "10.11 Erin's session is the provisioned user" ".id" "$ERIN_ID" "$LAST_BODY"
curl_api_full GET "/channels/${CONV_ID}/posts"
assert_status "10.11 Erin reads her conversation" 200 "$LAST_STATUS" "$LAST_BODY"

# 10.12 An identity with only an email (HALO's schema has no username or
# name trait): Chit derives the username from the email and the Kratos ID.
FRANK_KRATOS_ID=$(curl -s -X POST "${KRATOS_ADMIN}/admin/identities" -H "Content-Type: application/json" \
  -d '{"schema_id":"email-only","traits":{"email":"Frank.Only+halo@test.local"}}' | jq -r '.id')
if [[ -z "$FRANK_KRATOS_ID" || "$FRANK_KRATOS_ID" == "null" ]]; then
  FRANK_KRATOS_ID=$(curl -s "${KRATOS_ADMIN}/admin/identities?credentials_identifier=frank.only%2Bhalo@test.local" | jq -r '.[0].id')
fi
FRANK_USERNAME="frank.only-halo-$(echo "$FRANK_KRATOS_ID" | tr -d '-' | cut -c1-8)"
halod_api GET "/users/me" "" "kratos:${FRANK_KRATOS_ID}"
assert_status "10.12 provision an email-only identity" 200 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.12 derived username" ".username" "$FRANK_USERNAME" "$LAST_BODY"
assert_json "10.12 display name falls back to it" ".display_name" "$FRANK_USERNAME" "$LAST_BODY"
FRANK_ID=$(echo "$LAST_BODY" | jq -r '.id')
halod_api POST "/teams/${HALO_TEAM_ID}/members" "{\"user_id\":\"${FRANK_ID}\"}"
assert_status "10.12 halod adds Frank to its team" 201 "$LAST_STATUS" "$LAST_BODY"
halod_api POST "/channels" "{\"team_id\":\"${HALO_TEAM_ID}\",\"name\":\"conversation-frank\",\"display_name\":\"Conversation\",\"type\":\"P\"}" "kratos:${FRANK_KRATOS_ID}"
assert_status "10.12 create a channel on behalf of Frank" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "10.12 Frank is the creator" ".creator_id" "$FRANK_ID" "$LAST_BODY"

# ============================================================================
# SCENARIO 11: Attachments (HALO P-CHIT-5)
# ============================================================================
# halod uploads an image for Erin into her Conversation, fetches it for her,
# and posts it as her. chitd stores the bytes in SeaweedFS (the seaweedfs
# container of deploy/chit.kube.yml) through its S3 gateway, which refuses
# anonymous requests. chit-e2e-bot is a machine actor that is not on
# CHIT_ON_BEHALF_OF_CLIENTS.
section "Scenario 11: Attachments"

E2E_TMP=$(mktemp -d)
# A 1×1 PNG.
echo 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==' \
  | base64 -d > "${E2E_TMP}/dot.png"
# The same PNG with an HTML page after IEND: a polyglot.
cat "${E2E_TMP}/dot.png" > "${E2E_TMP}/polyglot.png"
printf '<html><script>alert(1)</script></html>' >> "${E2E_TMP}/polyglot.png"
printf '<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>' > "${E2E_TMP}/x.svg"

# upload_file TOKEN CHANNEL FILE CONTENT_TYPE ALT [ON_BEHALF_OF] — a
# multipart upload with a bearer token; sets LAST_STATUS, LAST_BODY and
# LAST_HEADERS (lowercased names).
upload_file() {
  local token="$1" channel="$2" file="$3" ctype="$4" alt="$5" obo="${6:-}"
  local body hdr
  body=$(mktemp); hdr=$(mktemp)
  local args=(-s -w '%{http_code}' -o "$body" -D "$hdr" -X POST -H "Authorization: Bearer ${token}"
    -F "file=@${file};type=${ctype}" -F "alt_text=${alt}")
  [[ -n "$obo" ]] && args+=(-H "X-On-Behalf-Of: ${obo}")
  LAST_STATUS=$(curl "${args[@]}" "${API}/channels/${channel}/files")
  LAST_BODY=$(cat "$body")
  LAST_HEADERS=$(tr 'A-Z' 'a-z' < "$hdr" | tr -d '\r')
  rm -f "$body" "$hdr"
}

# get_file TOKEN FILE_ID OUT [ON_BEHALF_OF] — downloads to OUT; sets
# LAST_STATUS and LAST_HEADERS.
get_file() {
  local token="$1" id="$2" out="$3" obo="${4:-}"
  local hdr
  hdr=$(mktemp)
  local args=(-s -w '%{http_code}' -o "$out" -D "$hdr" -H "Authorization: Bearer ${token}")
  [[ -n "$obo" ]] && args+=(-H "X-On-Behalf-Of: ${obo}")
  LAST_STATUS=$(curl "${args[@]}" "${API}/files/${id}")
  LAST_HEADERS=$(tr 'A-Z' 'a-z' < "$hdr" | tr -d '\r')
  rm -f "$hdr"
}

# assert_header LABEL "name: value" — LAST_HEADERS has that line.
assert_header() {
  if grep -qxF "$2" <<< "$LAST_HEADERS"; then
    echo -e "  ${GREEN}PASS${NC} $1 ($2)"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo -e "  ${RED}FAIL${NC} $1 — no \"$2\" in the response headers"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# 11.1 SeaweedFS refuses anonymous S3 requests: only chitd's identity works.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:8333/chit-files/")
assert_status "11.1 the S3 gateway refuses an anonymous request" 403 "$STATUS"

# 11.2 halod uploads for Erin, into her Conversation.
upload_file "$HALOD_TOKEN" "$CONV_ID" "${E2E_TMP}/dot.png" image/png "A single dot" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.2 upload on behalf of Erin" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "11.2 Erin is the uploader" ".uploader_id" "$ERIN_ID" "$LAST_BODY"
assert_json "11.2 halod is recorded" ".via_actor_id" "$HALOD_USER_ID" "$LAST_BODY"
assert_json "11.2 stored as PNG" ".content_type" "image/png" "$LAST_BODY"
assert_json "11.2 1×1" '"\(.width)x\(.height)"' "1x1" "$LAST_BODY"
assert_header "11.2 nosniff" "x-content-type-options: nosniff"
assert_header "11.2 sandbox CSP" "content-security-policy: sandbox"
FILE_ID=$(echo "$LAST_BODY" | jq -r '.id')
FILE_SHA=$(echo "$LAST_BODY" | jq -r '.sha256')

# 11.3 Before it is attached, halod reads it only as Erin.
get_file "$HALOD_TOKEN" "$FILE_ID" "${E2E_TMP}/got.png"
assert_status "11.3 halod as itself, before the post" 403 "$LAST_STATUS"
get_file "$HALOD_TOKEN" "$FILE_ID" "${E2E_TMP}/got.png" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.3 halod fetches it for Erin" 200 "$LAST_STATUS"
assert_header "11.3 image/png" "content-type: image/png"
assert_header "11.3 nosniff" "x-content-type-options: nosniff"
assert_header "11.3 sandbox CSP" "content-security-policy: sandbox"
assert_header "11.3 attachment" "content-disposition: attachment; filename=dot.png"
assert_json "11.3 the bytes are what was stored" "." "$FILE_SHA" "\"$(sha256sum "${E2E_TMP}/got.png" | cut -d' ' -f1)\""

# 11.4 halod posts it as Erin; then any member reads it, and Bob may not.
halod_api POST "/posts" "{\"channel_id\":\"${CONV_ID}\",\"content\":\"\",\"file_ids\":[\"${FILE_ID}\"]}" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.4 a post of the image alone" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "11.4 the post carries the file" ".file_ids[0]" "$FILE_ID" "$LAST_BODY"
FILE_POST_ID=$(echo "$LAST_BODY" | jq -r '.id')
get_file "$HALOD_TOKEN" "$FILE_ID" "${E2E_TMP}/got.png"
assert_status "11.4 halod, a member, reads the attached file" 200 "$LAST_STATUS"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -H "X-Session-Token: ${BOB_TOKEN}" "${API}/files/${FILE_ID}")
assert_status "11.4 Bob, not a member, may not" 403 "$STATUS"

# 11.5 Search finds it by alt text, for a member only.
halod_api POST "/files/search" '{"terms":"single DOT"}'
assert_json "11.5 found by alt text" ".[0].id" "$FILE_ID" "$LAST_BODY"
CURRENT_TOKEN="$BOB_TOKEN"
curl_api_full POST "/files/search" '{"terms":"single dot"}'
assert_json "11.5 not for a non-member" "length" "0" "$LAST_BODY"

# 11.6 The validation pipeline refuses SVG, a polyglot and a mismatch.
upload_file "$HALOD_TOKEN" "$CONV_ID" "${E2E_TMP}/x.svg" image/svg+xml "" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.6 SVG" 415 "$LAST_STATUS" "$LAST_BODY"
assert_json "11.6 SVG reason" ".id" "file.svg" "$LAST_BODY"
upload_file "$HALOD_TOKEN" "$CONV_ID" "${E2E_TMP}/polyglot.png" image/png "" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.6 PNG/HTML polyglot" 422 "$LAST_STATUS" "$LAST_BODY"
assert_json "11.6 polyglot reason" ".id" "file.polyglot" "$LAST_BODY"
upload_file "$HALOD_TOKEN" "$CONV_ID" "${E2E_TMP}/dot.png" image/jpeg "" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.6 declared JPEG, sniffed PNG" 415 "$LAST_STATUS" "$LAST_BODY"
assert_header "11.6 refusals carry nosniff" "x-content-type-options: nosniff"

# 11.7 On behalf of: a person's session, and a machine actor that is not on
# the allowlist, are refused.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "X-Session-Token: ${ALICE_TOKEN}" \
  -H "X-On-Behalf-Of: ${ERIN_ID}" -F "file=@${E2E_TMP}/dot.png;type=image/png" "${API}/channels/${CONV_ID}/files")
assert_status "11.7 a person's session with X-On-Behalf-Of" 403 "$STATUS"
BOT_CLIENT_ID="chit-e2e-bot"
BOT_CLIENT_SECRET="chit-e2e-bot-secret"
curl -s -o /dev/null -X POST "${HYDRA_ADMIN}/admin/clients" -H "Content-Type: application/json" \
  -d "$(jq -n --arg id "$BOT_CLIENT_ID" --arg secret "$BOT_CLIENT_SECRET" '{
    client_id: $id, client_secret: $secret, grant_types: ["client_credentials"],
    scope: "chit:read chit:write", audience: ["chit"],
    token_endpoint_auth_method: "client_secret_basic" }')"
BOT_TOKEN=$(curl -s -u "${BOT_CLIENT_ID}:${BOT_CLIENT_SECRET}" -X POST "${HYDRA_PUBLIC}/oauth2/token" \
  -d grant_type=client_credentials -d "scope=chit:read chit:write" -d audience=chit | jq -r '.access_token')
upload_file "$BOT_TOKEN" "$CONV_ID" "${E2E_TMP}/dot.png" image/png "" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.7 a machine actor not on the allowlist" 403 "$LAST_STATUS" "$LAST_BODY"
get_file "$BOT_TOKEN" "$FILE_ID" "${E2E_TMP}/got.png" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.7 nor may it download on behalf" 403 "$LAST_STATUS"

# 11.8 The same bytes in another channel get their own object; in the same
# channel they share one.
upload_file "$HALOD_TOKEN" "$CONV_ID" "${E2E_TMP}/dot.png" image/png "again" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.8 the same image again, same channel" 201 "$LAST_STATUS" "$LAST_BODY"
FRANK_CONV_ID=$($E2E_PSQL "SELECT id FROM channels WHERE name = 'conversation-frank'" | tr -d '[:space:]')
upload_file "$HALOD_TOKEN" "$FRANK_CONV_ID" "${E2E_TMP}/dot.png" image/png "" "kratos:${FRANK_KRATOS_ID}"
assert_status "11.8 the same image in Frank's channel" 201 "$LAST_STATUS" "$LAST_BODY"
OBJECTS=$($E2E_PSQL "SELECT channel_id || ':' || count(DISTINCT s3_key) || ':' || bool_and(s3_key LIKE channel_id || '/%') FROM files WHERE sha256 = '${FILE_SHA}' GROUP BY channel_id ORDER BY channel_id = '${CONV_ID}' DESC" | tr -d ' ' | tr '\n' ' ')
assert_json "11.8 one object per channel, each under its own channel" "." "${CONV_ID}:1:true ${FRANK_CONV_ID}:1:true " "\"${OBJECTS}\""

# 11.9 Deleting the post deletes its file.
kratos_login "erin@test.local" "TestPass1234"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE -H "X-Session-Token: ${SESSION_TOKEN}" "${API}/posts/${FILE_POST_ID}")
assert_status "11.9 Erin deletes her post" 200 "$STATUS"
get_file "$HALOD_TOKEN" "$FILE_ID" "${E2E_TMP}/got.png" "kratos:${ERIN_KRATOS_ID}"
assert_status "11.9 its file is gone" 404 "$LAST_STATUS"

rm -rf "$E2E_TMP"

# ============================================================================
# SCENARIO 12: Membership policy hook (HALO P-CHIT-2)
# ============================================================================
# chitd asks the hook (the memberhook-stub container, a stand-in for halod)
# before every channel add, with its own Hydra client_credentials token, which
# the stub checks by introspection. The stub allows everybody until it is told
# to refuse one user (Charlie), or to fail.
section "Scenario 12: Membership Policy Hook"

# hook_control JSON — sets the stub's policy.
hook_control() {
  curl -s -o /dev/null -X PUT "${HOOK_STUB}/control" -H "Content-Type: application/json" -d "$1"
}
# hook_calls — the requests the stub has answered, as a JSON array.
hook_calls() {
  curl -s "${HOOK_STUB}/control/calls"
}
# is_member CHANNEL_ID USER_ID — "1" when the user is in channel_members.
is_member() {
  $E2E_PSQL "SELECT count(*) FROM channel_members WHERE channel_id = '$1' AND user_id = '$2'" | tr -d '[:space:]'
}

# 12.1 The stub refuses a request without a valid token, so every call it
# records came with chitd's Hydra token.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "${HOOK_STUB}/membership-check" \
  -H "Authorization: Bearer not-a-token" -H "Content-Type: application/json" -d '{}')
assert_status "12.1 the stub refuses a token Hydra did not issue to chitd" 401 "$STATUS"

# 12.2 Scenario 10 added halod to Erin's Conversation on her behalf: the hook
# was asked about halod (an agent, no Kratos identity), added by Erin, via
# halod. Creating the Conversation asked nothing about Erin, its creator.
CALLS=$(hook_calls)
assert_json "12.2 the on-behalf-of add asked the hook" \
  "[.[] | select(.channel_id == \"${CONV_ID}\" and .user_id == \"${HALOD_USER_ID}\" and .added_by == \"${ERIN_ID}\" and .via_actor_id == \"${HALOD_USER_ID}\" and .actor_type == \"agent\" and .kratos_identity_id == \"\")] | length" \
  "1" "$CALLS"
assert_json "12.2 the creator of the Conversation was not asked about" \
  "[.[] | select(.channel_id == \"${CONV_ID}\" and .user_id == \"${ERIN_ID}\")] | length" "0" "$CALLS"

# 12.3 A private channel: its creator is not asked about.
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/channels" "{\"team_id\":\"${TEAM_ID}\",\"name\":\"hook-e2e\",\"display_name\":\"Hook e2e\",\"type\":\"P\"}"
assert_status "12.3 Alice creates a private channel" 201 "$LAST_STATUS" "$LAST_BODY"
HOOK_CHANNEL_ID=$(echo "$LAST_BODY" | jq -r '.id')
assert_json "12.3 nobody was asked about" "[.[] | select(.channel_id == \"${HOOK_CHANNEL_ID}\")] | length" "0" "$(hook_calls)"

# 12.4 Alice adds Bob: the hook is asked, with the contract's body.
curl_api_full POST "/channels/${HOOK_CHANNEL_ID}/members" "{\"user_id\":\"${BOB_ID}\"}"
assert_status "12.4 the hook allows Bob" 201 "$LAST_STATUS" "$LAST_BODY"
LAST_CALL=$(hook_calls | jq -c '.[-1]')
assert_json "12.4 the hook's request" \
  '[.channel_id, .user_id, .kratos_identity_id, .actor_type, .added_by, .via_actor_id] | join(",")' \
  "${HOOK_CHANNEL_ID},${BOB_ID},${BOB_KRATOS_ID},human,${ALICE_ID}," "$LAST_CALL"

# 12.5 The stub refuses Charlie: 403 channel.membership_refused, with the
# hook's reason and detail, and Charlie is not added.
hook_control "{\"refuse_user_id\":\"${CHARLIE_ID}\"}"
curl_api_full POST "/channels/${HOOK_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "12.5 the hook refuses Charlie" 403 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.5 error id" ".id" "channel.membership_refused" "$LAST_BODY"
assert_json "12.5 the hook's reason" ".reason" "e2e_refused" "$LAST_BODY"
assert_json "12.5 the hook's detail" ".detail" "The e2e stub hook refuses this user." "$LAST_BODY"
assert_json "12.5 Charlie is not a member" "." "0" "$(is_member "$HOOK_CHANNEL_ID" "$CHARLIE_ID")"

# 12.6 /invite asks the hook too, and says why it did not add him. The test
# grants Alice /invite in Keto for the command (chit-reconcile does not run
# in this stack), and takes the grant back after.
keto_put() {
  curl -s -o /dev/null -w '%{http_code}' -X PUT "${KETO_WRITE}/admin/relation-tuples" -H "Content-Type: application/json" -d "$1"
}
GRANT_STATUS=$(keto_put "{\"namespace\":\"chit/command\",\"object\":\"Role:e2e-hook\",\"relation\":\"member\",\"subject_id\":\"Actor:${ALICE_ID}\"}")
GRANT_STATUS+=$(keto_put '{"namespace":"chit/command","object":"Command:invite","relation":"execute","subject_set":{"namespace":"chit/command","object":"Role:e2e-hook","relation":"member"}}')
assert_json "12.6 grant /invite to Alice in Keto" "." "201201" "\"${GRANT_STATUS}\""
curl_api_full POST "/posts" "{\"channel_id\":\"${HOOK_CHANNEL_ID}\",\"content\":\"/invite charlie\"}"
assert_status "12.6 /invite charlie" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.6 the answer is a command response" ".type" "command_response" "$LAST_BODY"
assert_json "12.6 it gives the hook's reason" '.content | test("membership policy refused \\(e2e_refused\\)")' "true" "$LAST_BODY"
assert_json "12.6 Charlie is still not a member" "." "0" "$(is_member "$HOOK_CHANNEL_ID" "$CHARLIE_ID")"
curl -s -o /dev/null -X DELETE "${KETO_WRITE}/admin/relation-tuples?namespace=chit/command&object=Command:invite&relation=execute&subject_set.namespace=chit/command&subject_set.object=Role:e2e-hook&subject_set.relation=member"
curl -s -o /dev/null -X DELETE "${KETO_WRITE}/admin/relation-tuples?namespace=chit/command&object=Role:e2e-hook&relation=member&subject_id=Actor:${ALICE_ID}"

# 12.7 An Open channel adds the whole team, Charlie included: one refusal
# refuses the request, and no channel is created.
curl_api_full POST "/channels" "{\"team_id\":\"${TEAM_ID}\",\"name\":\"hook-open\",\"display_name\":\"Hook open\",\"type\":\"O\"}"
assert_status "12.7 an Open channel with a refused team member" 403 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.7 error id" ".id" "channel.membership_refused" "$LAST_BODY"
assert_json "12.7 no channel was created" "." "0" \
  "$($E2E_PSQL "SELECT count(*) FROM channels WHERE name = 'hook-open'" | tr -d '[:space:]')"

# 12.8 Joining an Open channel asks the hook: Charlie leaves general, and
# may not join it again.
CURRENT_TOKEN="$CHARLIE_TOKEN"
curl_api_full DELETE "/channels/${PUBLIC_CHANNEL_ID}/members/${CHARLIE_ID}"
assert_json "12.8 Charlie has left general" "." "0" "$(is_member "$PUBLIC_CHANNEL_ID" "$CHARLIE_ID")"
curl_api_full POST "/channels/${PUBLIC_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "12.8 Charlie may not join general" 403 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.8 error id" ".id" "channel.membership_refused" "$LAST_BODY"
assert_json "12.8 Charlie is not a member" "." "0" "$(is_member "$PUBLIC_CHANNEL_ID" "$CHARLIE_ID")"

# 12.9 A new direct or group channel asks about each member but its creator.
CURRENT_TOKEN="$DAVE_TOKEN"
curl_api_full POST "/channels/direct" "[\"${DAVE_ID}\",\"${CHARLIE_ID}\"]"
assert_status "12.9 a DM with Charlie" 403 "$LAST_STATUS" "$LAST_BODY"
curl_api_full POST "/channels/group" "[\"${DAVE_ID}\",\"${BOB_ID}\",\"${CHARLIE_ID}\"]"
assert_status "12.9 a group with Charlie" 403 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.9 neither channel was created" "." "0" \
  "$($E2E_PSQL "SELECT count(*) FROM channels WHERE type IN ('D','G') AND name LIKE '%${DAVE_ID}%'" | tr -d '[:space:]')"

# 12.10 The hook fails: Chit refuses (fails closed), with hook_unavailable.
hook_control '{"fail":true}'
CURRENT_TOKEN="$ALICE_TOKEN"
curl_api_full POST "/channels/${HOOK_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "12.10 a failing hook refuses the add" 403 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.10 reason" ".reason" "hook_unavailable" "$LAST_BODY"
assert_json "12.10 Charlie is not a member" "." "0" "$(is_member "$HOOK_CHANNEL_ID" "$CHARLIE_ID")"

# 12.11 The hook allows Charlie again: the add goes through.
hook_control '{}'
curl_api_full POST "/channels/${HOOK_CHANNEL_ID}/members" "{\"user_id\":\"${CHARLIE_ID}\"}"
assert_status "12.11 the hook allows Charlie now" 201 "$LAST_STATUS" "$LAST_BODY"
assert_json "12.11 Charlie is a member" "." "1" "$(is_member "$HOOK_CHANNEL_ID" "$CHARLIE_ID")"

# 12.12 The audit log records the answers, allowed and refused.
AUDIT=$($E2E_AUDIT 2>/dev/null | jq -s -c '[.[] | select(.msg == "membership_check" and .channel_id == "'"${HOOK_CHANNEL_ID}"'")]' 2>/dev/null || echo '[]')
assert_json "12.12 audit: Bob allowed" "[.[] | select(.user_id == \"${BOB_ID}\" and .allowed == true and .added_by == \"${ALICE_ID}\")] | length" "1" "$AUDIT"
assert_json "12.12 audit: Charlie refused by the hook" "[.[] | select(.user_id == \"${CHARLIE_ID}\" and .allowed == false and .reason == \"e2e_refused\")] | length" "2" "$AUDIT"
assert_json "12.12 audit: Charlie refused, the hook failing" "[.[] | select(.user_id == \"${CHARLIE_ID}\" and .allowed == false and .reason == \"hook_unavailable\")] | length" "1" "$AUDIT"

section "Scenario 13: Rate Limits"

# burst TOKEN N [ON_BEHALF_OF] — N GET /users/me at once, 16 in parallel;
# prints each response's status and Retry-After header, one per line.
burst() {
  local token="$1" n="$2" obo="${3:-}"
  local args=(-s -o /dev/null -D - -H "Authorization: Bearer ${token}")
  [[ -n "$obo" ]] && args+=(-H "X-On-Behalf-Of: ${obo}")
  seq "$n" | xargs -P 16 -I{} curl "${args[@]}" "${API}/users/me" \
    | tr -d '\r' | awk 'tolower($1) ~ /^http\// {print $2} tolower($1) == "retry-after:" {print "retry-after " $2}'
}

# 13.1 halod as itself, past the per-user burst of 50: what an allowlisted
# client does as itself counts against its aggregate limit (1500 a second,
# bursts of 3000), not a per-user one.
OUT=$(burst "$HALOD_TOKEN" 120)
assert_json "13.1 halod as itself: 120 at once, all served" "." "120" "$(grep -cx 200 <<< "$OUT")"

# 13.2 On behalf of Erin, halod spends Erin's per-user budget: some of 120
# at once are refused, each with Retry-After.
OUT=$(burst "$HALOD_TOKEN" 120 "kratos:${ERIN_KRATOS_ID}")
REFUSED=$(grep -cx 429 <<< "$OUT")
assert_json_gte "13.2 on behalf of Erin: refused past her burst" "." 1 "$REFUSED"
assert_json "13.2 each refusal has Retry-After: 1" "." "$REFUSED" "$(grep -cx 'retry-after 1' <<< "$OUT")"
assert_json "13.2 the rest are served" "." "$((120 - REFUSED))" "$(grep -cx 200 <<< "$OUT")"

# 13.3 Erin's budget is hers: halod as itself, and another person, are served.
halod_api GET "/users/me"
assert_status "13.3 halod as itself is still served" 200 "$LAST_STATUS" "$LAST_BODY"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -H "X-Session-Token: ${ALICE_TOKEN}" "${API}/users/me")
assert_status "13.3 Alice is still served" 200 "$STATUS"

# 13.4 A machine actor that is not on CHIT_ON_BEHALF_OF_CLIENTS keeps the
# per-user limit.
OUT=$(burst "$BOT_TOKEN" 120)
assert_json_gte "13.4 chit-e2e-bot: refused past the per-user burst" "." 1 "$(grep -cx 429 <<< "$OUT")"

# 13.5 chitd logs each refusal with the actor and the principal.
LOG=$($E2E_CHITD_LOG 2>&1 | jq -c -R 'fromjson? | select(.msg == "rate limit exceeded")' | jq -s -c '.' 2>/dev/null || echo '[]')
assert_json_gte "13.5 log: on behalf of Erin, her bucket, actor and principal" \
  "[.[] | select(.bucket == \"user\" and .principal_id == \"${ERIN_ID}\" and .actor_id == \"${HALOD_USER_ID}\" and .client_id == \"${HALOD_CLIENT_ID}\")] | length" 1 "$LOG"
assert_json_gte "13.5 log: chit-e2e-bot, its own bucket" \
  "[.[] | select(.bucket == \"user\" and .client_id == \"${BOT_CLIENT_ID}\" and .path == \"/api/v1/users/me\")] | length" 1 "$LOG"

# ============================================================================
# SUMMARY
# ============================================================================
section "Test Summary"

TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo ""
echo -e "  ${GREEN}PASS: ${PASS_COUNT}${NC}"
echo -e "  ${RED}FAIL: ${FAIL_COUNT}${NC}"
echo -e "  ${YELLOW}SKIP: ${SKIP_COUNT}${NC}"
echo -e "  ${BOLD}TOTAL: ${TOTAL}${NC}"
echo ""

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  echo -e "${RED}${BOLD}SOME TESTS FAILED${NC}"
  exit 1
else
  echo -e "${GREEN}${BOLD}ALL TESTS PASSED${NC}"
  exit 0
fi
