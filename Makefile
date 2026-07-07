.PHONY: build build-mcp build-reconcile build-claude build-tui build-all run test test-tui test-all test-container test-e2e test-e2e-tui test-e2e-clean lint lint-tui check-deps validate-spec cue-validate reconcile migrate-up migrate-down kube-migrate kube-build kube-up kube-down kube-restart kube-logs kube-clean uat-up uat-seed-kratos uat-down uat-clean docs-dev clean

BINARY=bin/chitd
MCP_BINARY=bin/chit-mcp
RECONCILE_BINARY=bin/chit-reconcile
CLAUDE_BINARY=bin/chit-claude
KUBE_FILE=deploy/chit.kube.yml
UAT_KUBE_FILE=deploy/chit-uat.kube.yml
MIGRATE_URL?=postgres://chit:chit@localhost:5432/chit?sslmode=disable
CHIT_IMAGE?=localhost/chit:latest

E2E_POD=chit-e2e
E2E_IMAGE=chit-e2e:latest
PG_IMAGE=docker.io/library/postgres:17-alpine


# Server-module Go invocations run with GOWORK=off so each module in the
# workspace (root + clients/chit-tui) resolves standalone from its own go.mod.
build:
	GOWORK=off go build -o $(BINARY) ./cmd/chitd

build-mcp:
	GOWORK=off go build -o $(MCP_BINARY) ./cmd/chit-mcp

build-reconcile:
	GOWORK=off go build -o $(RECONCILE_BINARY) ./cmd/chit-reconcile

build-claude:
	GOWORK=off go build -o $(CLAUDE_BINARY) ./cmd/chit-claude

build-tui:
	$(MAKE) -C clients/chit-tui build

build-all: build build-mcp build-reconcile build-claude build-tui

validate-spec:
	go run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml

run: build
	./$(BINARY)

test:
	GOWORK=off go test -race -count=1 ./...

test-tui:
	$(MAKE) -C clients/chit-tui test

test-all: test test-tui

test-container:
	podman build -t chit-test:latest -f Containerfiles/Containerfile.test .
	podman run --rm chit-test:latest

test-e2e:
	podman build -t $(E2E_IMAGE) -f Containerfiles/Containerfile.e2e .
	-podman pod rm -f $(E2E_POD) 2>/dev/null
	podman pod create --name $(E2E_POD) --share net
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-pg \
		-e POSTGRES_USER=chit -e POSTGRES_PASSWORD=chit -e POSTGRES_DB=chit \
		$(PG_IMAGE)
	podman run --rm --pod $(E2E_POD) --name $(E2E_POD)-tests \
		-e CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable \
		$(E2E_IMAGE) ; \
	EXIT_CODE=$$? ; \
	podman pod rm -f $(E2E_POD) ; \
	exit $$EXIT_CODE

test-e2e-clean:
	-podman pod rm -f $(E2E_POD) 2>/dev/null

lint:
	golangci-lint run

# Dependency-boundary guards: the client binaries (chit-claude, chit-mcp)
# talk to chitd over HTTP/WS only and must never link server-side deps;
# chitd must not link the MCP SDK; TUI (charmbracelet) deps must never
# enter the server module. Run with GOWORK=off so each check reflects the
# root module standalone.
check-deps:
	@echo "checking cmd/chit-claude stays a pure HTTP/WS client..."
	@! GOWORK=off go list -deps ./cmd/chit-claude | grep -qE 'jackc/pgx|cuelang|modelcontextprotocol|kin-openapi|nats-io' \
		|| (echo 'FAIL: chit-claude gained a server-side dependency' && exit 1)
	@echo "checking cmd/chit-mcp stays a pure HTTP/WS client..."
	@! GOWORK=off go list -deps ./cmd/chit-mcp | grep -qE 'jackc/pgx|cuelang|kin-openapi|nats-io' \
		|| (echo 'FAIL: chit-mcp gained a server-side dependency' && exit 1)
	@echo "checking cmd/chit-reconcile links no runtime server deps..."
	@! GOWORK=off go list -deps ./cmd/chit-reconcile | grep -qE 'jackc/pgx|modelcontextprotocol|kin-openapi|nats-io' \
		|| (echo 'FAIL: chit-reconcile gained a runtime server dependency' && exit 1)
	@echo "checking cmd/chitd does not link the MCP SDK..."
	@! GOWORK=off go list -deps ./cmd/chitd | grep -q modelcontextprotocol \
		|| (echo 'FAIL: chitd gained the MCP SDK' && exit 1)
	@echo "checking TUI deps stay out of the server module..."
	@! GOWORK=off go list -m all | grep -qi charmbracelet \
		|| (echo 'FAIL: charmbracelet deps leaked into the server module' && exit 1)
	@echo "check-deps: all dependency boundaries hold"

lint-tui:
	$(MAKE) -C clients/chit-tui lint

test-e2e-tui:
	$(MAKE) -C clients/chit-tui test-e2e

## --- CUE / Reconciler ---

cue-validate:
	go run cuelang.org/go/cmd/cue@latest eval ./auth/...

reconcile: build-reconcile
	./$(RECONCILE_BINARY)

## --- Database Migrations ---

migrate-up:
	migrate -database "$(MIGRATE_URL)" -path migrations up

migrate-down:
	migrate -database "$(MIGRATE_URL)" -path migrations down

# The kube pod runs Kratos/Keto/chit migrations automatically via init and
# migrate containers on `make kube-up`. This target re-applies the chit schema
# migrations against the running pod's PostgreSQL — useful after adding new
# migration files without restarting the pod. Requires no local migrate binary.
kube-migrate:
	podman run --rm --network host -v $(CURDIR)/migrations:/migrations:ro,Z \
		docker.io/migrate/migrate:v4.15.0 \
		-database "$(MIGRATE_URL)" -path /migrations up

## --- Podman Kube ---

kube-build:
	podman build -t $(CHIT_IMAGE) -f deploy/Containerfile .

kube-up: kube-build
	sed "s|__PROJECT_ROOT__|$(CURDIR)|g" $(KUBE_FILE) | podman kube play --network host -

kube-down:
	sed "s|__PROJECT_ROOT__|$(CURDIR)|g" $(KUBE_FILE) | podman kube down -

kube-restart: kube-down kube-up

kube-logs:
	podman pod logs -f chit-app

kube-log-%:
	podman logs -f chit-app-$*

kube-clean: kube-down
	-podman volume rm chit-pgdata chit-zincdata 2>/dev/null

## --- UAT (Manual Acceptance Testing) ---

uat-up: kube-build
	podman build -t $(E2E_IMAGE) -f Containerfiles/Containerfile.e2e .
	sed "s|__PROJECT_ROOT__|$(CURDIR)|g" $(UAT_KUBE_FILE) | podman kube play --network host -
	@echo ""
	@echo "UAT environment starting..."
	@echo "  Server: http://localhost:8065/api/v1/system/ping"
	@echo "  Proxy:  http://localhost:4455/api/v1/system/ping"
	@echo "  Logs:   podman logs -f chit-uat-app-chitd"
	@echo ""
	@echo "Once services are ready, seed Kratos identities for TUI login:"
	@echo "  make uat-seed-kratos"
	@echo ""

uat-seed-kratos:
	@echo "Seeding Kratos identities (requires UAT pod running)..."
	KRATOS_ADMIN_URL=http://localhost:4434 CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable go run scripts/seed-kratos/main.go

uat-down:
	sed "s|__PROJECT_ROOT__|$(CURDIR)|g" $(UAT_KUBE_FILE) | podman kube down -

uat-clean: uat-down
	-podman volume rm chit-uat-pgdata chit-uat-zincdata 2>/dev/null

## --- Docs ---

docs-dev:
	cd docs && bun --bun run dev

clean:
	rm -rf bin/
